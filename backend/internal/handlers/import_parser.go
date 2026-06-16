package handlers

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type importQuestion struct {
	Text    string
	Options []optionInput
}

// questionStart — «1. текст» или «1) текст»
var questionStart = regexp.MustCompile(`^\d+[\.\)]\s*(.*)$`)

// optionLetter — «A) текст», «а) текст»
var optionLetter = regexp.MustCompile(`^([A-Za-zА-Яа-я])[\.\)]\s*(.+)$`)

// ParseImportText разбирает текст с вопросами.
//
// Основной формат (варианты — строки с «*», правильные отметьте потом в админке):
//
//	1. Текст вопроса?
//	* первый вариант
//	* второй вариант
//
//	2. Следующий вопрос?
//	* вариант A
//	* вариант B
//
// Дополнительно: «+ текст» — сразу правильный ответ; «- текст» — неправильный.
// Между вопросами — пустая строка или новый номер.
func ParseImportText(raw string) ([]importQuestion, error) {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("пустой текст")
	}

	var out []importQuestion
	var cur *importQuestion
	hasOptions := false

	flush := func() error {
		if cur == nil {
			return nil
		}
		cur.Text = strings.TrimSpace(cur.Text)
		if cur.Text == "" {
			return fmt.Errorf("вопрос №%d без текста", len(out)+1)
		}
		if len(cur.Options) < 2 {
			return fmt.Errorf("вопрос «%s»: нужно минимум 2 варианта ответа", truncateRunes(cur.Text, 40))
		}
		out = append(out, *cur)
		cur = nil
		hasOptions = false
		return nil
	}

	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "---" {
			// Пустая строка после текста вопроса (до вариантов) — норма, не закрываем вопрос.
			if cur != nil && len(cur.Options) >= 2 {
				if err := flush(); err != nil {
					return nil, err
				}
			}
			continue
		}

		if m := questionStart.FindStringSubmatch(line); m != nil {
			if err := flush(); err != nil {
				return nil, err
			}
			cur = &importQuestion{Options: []optionInput{}}
			cur.Text = strings.TrimSpace(m[1])
			hasOptions = false
			continue
		}

		if cur == nil {
			return nil, fmt.Errorf("ожидается нумерация вопроса (1. … или 1) …), получено: %s", truncateRunes(line, 50))
		}

		// * — вариант ответа (маркер списка, не «правильный»)
		if strings.HasPrefix(line, "*") {
			text := strings.TrimSpace(line[1:])
			if text == "" {
				return nil, fmt.Errorf("пустой вариант ответа в вопросе «%s»", truncateRunes(cur.Text, 40))
			}
			cur.Options = append(cur.Options, optionInput{Text: text, IsCorrect: false})
			hasOptions = true
			continue
		}
		// + — явно правильный (если хотите отметить сразу при импорте)
		if strings.HasPrefix(line, "+") {
			text := strings.TrimSpace(line[1:])
			if text == "" {
				return nil, fmt.Errorf("пустой вариант ответа в вопросе «%s»", truncateRunes(cur.Text, 40))
			}
			cur.Options = append(cur.Options, optionInput{Text: text, IsCorrect: true})
			hasOptions = true
			continue
		}
		if strings.HasPrefix(line, "-") {
			text := strings.TrimSpace(line[1:])
			if text == "" {
				return nil, fmt.Errorf("пустой вариант ответа в вопросе «%s»", truncateRunes(cur.Text, 40))
			}
			cur.Options = append(cur.Options, optionInput{Text: text, IsCorrect: false})
			hasOptions = true
			continue
		}

		if m := optionLetter.FindStringSubmatch(line); m != nil {
			text, isCorrect := stripCorrectMarkers(strings.TrimSpace(m[2]))
			if text == "" {
				return nil, fmt.Errorf("пустой вариант ответа в вопросе «%s»", truncateRunes(cur.Text, 40))
			}
			cur.Options = append(cur.Options, optionInput{Text: text, IsCorrect: isCorrect})
			hasOptions = true
			continue
		}

		// Продолжение текста вопроса (до первого варианта)
		if !hasOptions {
			if cur.Text == "" {
				cur.Text = line
			} else {
				cur.Text += "\n" + line
			}
			continue
		}

		return nil, fmt.Errorf("непонятная строка в вопросе «%s»: %s", truncateRunes(cur.Text, 40), truncateRunes(line, 50))
	}

	if err := flush(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("не найдено ни одного вопроса")
	}
	return out, nil
}

// stripCorrectMarkers убирает * вокруг текста и возвращает (text, isCorrect).
func stripCorrectMarkers(s string) (string, bool) {
	s = strings.TrimSpace(s)
	isCorrect := false
	for strings.HasPrefix(s, "*") && strings.HasSuffix(s, "*") && len(s) > 1 {
		s = strings.TrimSpace(s[1 : len(s)-1])
		isCorrect = true
	}
	if strings.HasPrefix(s, "*") {
		s = strings.TrimSpace(s[1:])
		isCorrect = true
	}
	if strings.HasSuffix(s, "*") {
		s = strings.TrimSpace(s[:len(s)-1])
		isCorrect = true
	}
	return strings.TrimSpace(s), isCorrect
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

type importJSON struct {
	Questions []struct {
		Text    string `json:"text"`
		Options []struct {
			Text      string `json:"text"`
			IsCorrect bool   `json:"isCorrect"`
		} `json:"options"`
	} `json:"questions"`
}

func ParseImportJSON(raw string) ([]importQuestion, error) {
	var doc importJSON
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, fmt.Errorf("некорректный JSON: %w", err)
	}
	if len(doc.Questions) == 0 {
		return nil, fmt.Errorf("в JSON нет questions")
	}
	out := make([]importQuestion, 0, len(doc.Questions))
	for i, q := range doc.Questions {
		text := strings.TrimSpace(q.Text)
		if text == "" {
			return nil, fmt.Errorf("вопрос %d: пустой text", i+1)
		}
		if len(q.Options) < 2 {
			return nil, fmt.Errorf("вопрос %d: нужно минимум 2 options", i+1)
		}
		iq := importQuestion{Text: text, Options: make([]optionInput, 0, len(q.Options))}
		for _, o := range q.Options {
			t := strings.TrimSpace(o.Text)
			if t == "" {
				return nil, fmt.Errorf("вопрос %d: пустой текст варианта", i+1)
			}
			iq.Options = append(iq.Options, optionInput{Text: t, IsCorrect: o.IsCorrect})
		}
		out = append(out, iq)
	}
	return out, nil
}

func parseImportPayload(raw string) ([]importQuestion, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("пустой текст")
	}
	if strings.HasPrefix(raw, "{") || strings.HasPrefix(raw, "[") {
		return ParseImportJSON(raw)
	}
	return ParseImportText(raw)
}
