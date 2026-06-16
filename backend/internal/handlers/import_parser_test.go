package handlers

import "testing"

func TestParseImportText_blankLineBeforeOptions(t *testing.T) {
	text := `1. Файловая система подразделяется на

* внутреннюю и внешнюю
* простую и сложную
* простую и иерархическую
* динамическую и статическую

2. Процесс, в результате которого в несколько раз уменьшается размер файлов
* сжатие информации
* распаковка
* разархивация
* удаление информации`
	qs, err := ParseImportText(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 2 {
		t.Fatalf("want 2 questions, got %d", len(qs))
	}
	if len(qs[0].Options) != 4 {
		t.Fatalf("q1 options: %d", len(qs[0].Options))
	}
}
