package proxy

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"mcpscope/internal/intercept"
	"mcpscope/internal/store"
)

type Config struct {
	Server     string
	ServerName string
	Port       int
	Transport  string
	Store      store.TraceStore
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
}

func Run(ctx context.Context, cfg Config) error {
	switch cfg.Transport {
	case "stdio":
		return runStdio(ctx, cfg)
	case "http":
		return runHTTP(ctx, cfg)
	default:
		return fmt.Errorf("unsupported transport %q", cfg.Transport)
	}
}

func runStdio(ctx context.Context, cfg Config) error {
	cmd := exec.CommandContext(ctx, cfg.Server)
	cmd.Stderr = cfg.Stderr
	cmd.Env = append(os.Environ(), fmt.Sprintf("MCPSCOPE_PROXY_PORT=%d", cfg.Port))

	serverIn, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("create subprocess stdin pipe: %w", err)
	}

	serverOut, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create subprocess stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start subprocess: %w", err)
	}

	copyErr := make(chan error, 2)

	go func() {
		copyErr <- forwardStdio(ctx, cfg, cfg.Stdin, serverIn, "client_to_server")
	}()

	go func() {
		copyErr <- forwardStdio(ctx, cfg, serverOut, cfg.Stdout, "server_to_client")
	}()

	var firstErr error
	for i := 0; i < 2; i++ {
		if err := <-copyErr; err != nil && !errors.Is(err, io.EOF) && firstErr == nil {
			firstErr = err
		}
	}

	waitErr := cmd.Wait()

	if firstErr != nil {
		return firstErr
	}

	if waitErr != nil {
		return fmt.Errorf("subprocess exited with error: %w", waitErr)
	}

	return nil
}

func forwardStdio(ctx context.Context, cfg Config, src io.Reader, dst io.Writer, direction string) error {
	reader := bufio.NewReader(src)
	writeCloser, canClose := dst.(io.WriteCloser)

	for {
		receivedAt := time.Now()
		frame, err := readFrame(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				if canClose {
					return writeCloser.Close()
				}
				return nil
			}
			return err
		}

		if _, err := dst.Write(frame.header); err != nil {
			return fmt.Errorf("write frame header: %w", err)
		}
		if _, err := dst.Write(frame.payload); err != nil {
			return fmt.Errorf("write frame payload: %w", err)
		}
		if flusher, ok := dst.(interface{ Flush() error }); ok {
			if err := flusher.Flush(); err != nil {
				return fmt.Errorf("flush frame: %w", err)
			}
		}

		sentAt := time.Now()
		if err := captureAndPersist(ctx, cfg, "stdio", direction, receivedAt, sentAt, frame.payload); err != nil {
			return err
		}
	}
}

type frame struct {
	header  []byte
	payload []byte
}

func readFrame(reader *bufio.Reader) (frame, error) {
	var header bytes.Buffer
	contentLength := -1

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) && header.Len() == 0 {
				return frame{}, io.EOF
			}
			return frame{}, fmt.Errorf("read frame header: %w", err)
		}

		header.Write(line)
		trimmed := strings.TrimRight(string(line), "\r\n")
		if trimmed == "" {
			break
		}

		name, value, found := strings.Cut(trimmed, ":")
		if !found {
			continue
		}

		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			parsed, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return frame{}, fmt.Errorf("parse content length: %w", err)
			}
			contentLength = parsed
		}
	}

	if contentLength < 0 {
		return frame{}, fmt.Errorf("missing Content-Length header")
	}

	payload := make([]byte, contentLength)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return frame{}, fmt.Errorf("read frame payload: %w", err)
	}

	return frame{
		header:  header.Bytes(),
		payload: payload,
	}, nil
}

func runHTTP(ctx context.Context, cfg Config) error {
	upstreamPort := cfg.Port + 1
	if err := validateUpstreamPort(upstreamPort); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, cfg.Server)
	cmd.Stdout = cfg.Stderr
	cmd.Stderr = cfg.Stderr
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("PORT=%d", upstreamPort),
		fmt.Sprintf("MCPSCOPE_PROXY_PORT=%d", cfg.Port),
		fmt.Sprintf("MCPSCOPE_UPSTREAM_PORT=%d", upstreamPort),
	)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start subprocess: %w", err)
	}

	targetBaseURL, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", upstreamPort))
	if err != nil {
		return fmt.Errorf("build upstream url: %w", err)
	}

	var mu sync.Mutex
	client := &http.Client{}
	server := &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.Port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}

			requestBody, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "failed to read request body", http.StatusBadRequest)
				return
			}
			defer r.Body.Close()

			requestReceivedAt := time.Now()

			upstreamURL := *targetBaseURL
			upstreamURL.Path = r.URL.Path
			upstreamURL.RawQuery = r.URL.RawQuery

			req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL.String(), bytes.NewReader(requestBody))
			if err != nil {
				http.Error(w, "failed to build upstream request", http.StatusInternalServerError)
				return
			}

			req.Header = r.Header.Clone()
			req.Header.Del("Host")

			mu.Lock()
			resp, err := client.Do(req)
			mu.Unlock()
			requestSentAt := time.Now()
			if err := captureAndPersist(r.Context(), cfg, "http", "client_to_server", requestReceivedAt, requestSentAt, requestBody); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err != nil {
				http.Error(w, fmt.Sprintf("proxy upstream request: %v", err), http.StatusBadGateway)
				return
			}
			defer resp.Body.Close()

			responseReceivedAt := time.Now()
			responseBody, err := io.ReadAll(resp.Body)
			if err != nil {
				http.Error(w, "failed to read upstream response", http.StatusBadGateway)
				return
			}

			for key, values := range resp.Header {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}

			w.WriteHeader(resp.StatusCode)
			if _, err := w.Write(responseBody); err != nil {
				return
			}

			responseSentAt := time.Now()
			_ = captureAndPersist(r.Context(), cfg, "http", "server_to_client", responseReceivedAt, responseSentAt, responseBody)
		}),
	}

	serverErr := make(chan error, 1)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	go func() {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	waitErr := make(chan error, 1)
	go func() {
		waitErr <- cmd.Wait()
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("http proxy server failed: %w", err)
		}
		if err := <-waitErr; err != nil {
			return fmt.Errorf("subprocess exited with error: %w", err)
		}
		return nil
	case err := <-waitErr:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		if err != nil {
			return fmt.Errorf("subprocess exited with error: %w", err)
		}
		if err := <-serverErr; err != nil {
			return fmt.Errorf("http proxy server failed: %w", err)
		}
		return nil
	}
}

func captureAndPersist(ctx context.Context, cfg Config, transport, direction string, receivedAt, sentAt time.Time, payload []byte) error {
	event := intercept.Capture(transport, direction, receivedAt, sentAt, payload)

	if err := intercept.EmitLog(cfg.Stderr, event); err != nil {
		return err
	}

	if cfg.Store == nil {
		return nil
	}

	return cfg.Store.Insert(ctx, store.Trace{
		ID:           intercept.NewUUID(),
		TraceID:      event.TraceID,
		ServerName:   cfg.ServerName,
		Method:       event.Method,
		ParamsHash:   event.ParamsHash,
		ResponseHash: event.ResponseHash,
		LatencyMs:    event.LatencyMs,
		IsError:      event.IsError,
		ErrorMessage: event.ErrorMessage,
		CreatedAt:    time.Unix(0, event.ReceivedAtUnixN).UTC(),
	})
}

func validateUpstreamPort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("derived upstream port %d is out of range", port)
	}
	return nil
}
