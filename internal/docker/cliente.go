// Package docker: cliente mínimo de la API de Docker por el socket unix.
// Sin librerías externas y sin llamar al comando "docker": solo HTTP.
// Los workspaces GNS3 viven en Docker; la plataforma vive en Podman.
package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const SocketPorDefecto = "/var/run/docker.sock"

// Cliente se crea una vez y se comparte por puntero
type Cliente struct {
	http *http.Client
}

func Nuevo(socket string) *Cliente {
	if socket == "" {
		socket = SocketPorDefecto
	}
	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socket)
		},
		MaxIdleConns:    10,
		IdleConnTimeout: 30 * time.Second,
	}
	return &Cliente{http: &http.Client{Transport: tr}}
}

// peticion hace la llamada HTTP a Docker y decodifica la respuesta en destino (si no es nil)
func (c *Cliente) peticion(ctx context.Context, metodo, ruta string, cuerpo io.Reader, destino any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, metodo, "http://docker"+ruta, cuerpo)
	if err != nil {
		return 0, err
	}
	if cuerpo != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("no se pudo hablar con Docker: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var e struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return resp.StatusCode, fmt.Errorf("docker respondió %d: %s", resp.StatusCode, strings.TrimSpace(e.Message))
	}
	if destino != nil {
		if err := json.NewDecoder(resp.Body).Decode(destino); err != nil && err != io.EOF {
			return resp.StatusCode, err
		}
	}
	return resp.StatusCode, nil
}

// Ping confirma que el socket responde
func (c *Cliente) Ping(ctx context.Context) error {
	_, err := c.peticion(ctx, http.MethodGet, "/_ping", nil, nil)
	return err
}

// Version regresa la versión del motor Docker del host
func (c *Cliente) Version(ctx context.Context) (string, error) {
	var v struct {
		Version string `json:"Version"`
	}
	_, err := c.peticion(ctx, http.MethodGet, "/version", nil, &v)
	return v.Version, err
}
