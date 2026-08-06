package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os/exec"

	"github.com/go-chi/chi/v5"
)

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run()
}

func jsonDecode(r *http.Request, v any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(v)
}

func urlParam(r *http.Request, name string) string {
	return chi.URLParam(r, name)
}
