package handlers

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/mountest/backend/internal/httpx"
)

// UploadHandler принимает картинки от админки и раздаёт их обратно по публичной ссылке.
// Файлы лежат на диске в UploadDir (named volume в проде).
type UploadHandler struct {
	Dir string
}

// Лимит на размер тела запроса (немного больше 5 МБ, чтобы успеть выкинуть 413 до парсинга формы).
const (
	maxImageSize  = 5 << 20
	maxBodyMargin = 1 << 20
)

// Whitelist content-type → расширение.
var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/jpg":  ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

// safeFileName: только то, что мы сами генерируем (UUID + расширение из whitelist).
var safeFileName = regexp.MustCompile(`^[a-f0-9-]{36}\.(jpg|png|webp)$`)

// EnsureDir создаёт каталог под загрузки и проверяет, что в него реально можно писать.
func (h *UploadHandler) EnsureDir() error {
	if err := os.MkdirAll(h.Dir, 0o755); err != nil {
		return fmt.Errorf("mkdir upload dir: %w", err)
	}
	probe := filepath.Join(h.Dir, ".write_probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
		return fmt.Errorf("upload dir %q not writable: %w", h.Dir, err)
	}
	if err := os.Remove(probe); err != nil {
		return fmt.Errorf("remove upload probe: %w", err)
	}
	return nil
}

// Upload — POST /api/admin/upload. Требует авторизованного админа (любой роли).
// Принимает multipart/form-data с полем "file". Возвращает {"url": "/api/uploads/<file>"}.
func (h *UploadHandler) Upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImageSize+maxBodyMargin)
	if err := r.ParseMultipartForm(maxImageSize); err != nil {
		log.Printf("upload: ParseMultipartForm: %v", err)
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			httpx.WriteError(w, http.StatusRequestEntityTooLarge, "Файл слишком большой (макс. 5 МБ)")
			return
		}
		httpx.WriteError(w, http.StatusBadRequest, "Не удалось разобрать форму загрузки")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Файл не найден в запросе (ожидается поле \"file\")")
		return
	}
	defer file.Close()

	if header.Size > 0 && header.Size > maxImageSize {
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "Файл слишком большой (макс. 5 МБ)")
		return
	}

	// Читаем тело части в память (лимит 5 МБ) — так не зависим от Seek у multipart.File.
	payload, err := io.ReadAll(io.LimitReader(file, maxImageSize+1))
	if err != nil {
		log.Printf("upload: read body: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить файл")
		return
	}
	if len(payload) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "Пустой файл")
		return
	}
	if len(payload) > maxImageSize {
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "Файл слишком большой (макс. 5 МБ)")
		return
	}

	ct := strings.ToLower(http.DetectContentType(payload))
	ext, ok := allowedImageTypes[ct]
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "Допустимы только JPG, PNG или WebP")
		return
	}

	name := uuid.NewString() + ext
	dstPath := filepath.Join(h.Dir, name)
	if err := os.WriteFile(dstPath, payload, 0o644); err != nil {
		log.Printf("upload: writefile %q: %v", dstPath, err)
		httpx.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить файл")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, map[string]string{
		"url": "/api/uploads/" + name,
	})
}

// Serve — GET /api/uploads/{name}. Публичная отдача (ученикам нужно показывать картинки во время попытки).
func (h *UploadHandler) Serve(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !safeFileName.MatchString(name) {
		httpx.WriteError(w, http.StatusNotFound, "not found")
		return
	}
	p := filepath.Join(h.Dir, name)
	// На всякий случай — гарантируем, что итоговый путь действительно внутри Dir.
	abs, err := filepath.Abs(p)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not found")
		return
	}
	dirAbs, err := filepath.Abs(h.Dir)
	if err != nil || !strings.HasPrefix(abs, dirAbs+string(os.PathSeparator)) {
		httpx.WriteError(w, http.StatusNotFound, "not found")
		return
	}
	if _, err := os.Stat(p); err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not found")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, p)
}
