// Package i18n holds the machine's user-facing messages, formatted in the learner's language.
package i18n

import "strings"

// Pack holds every message the machine prints to the learner.
// Formats take the same arguments in every language.
type Pack struct {
	CheckFailed   string // the queued check failed; arg: reason
	WeekRate      string // last week's summary; args: rate per 100 words, word count
	WeekPrev      string // comparison with the week before; arg: rate
	WeekTop       string // the most frequent rule of the week; args: rule, count
	DrillsRipe    string // a topic reached its threshold; args: list, id, id
	CardsDue      string // spaced-repetition cards are due; arg: count
	SpeechPending string // speech recordings wait for review; arg: count
	NthTime       string // the rule repeated for the n-th time; arg: count
	MoreInDB      string // more corrections than shown; arg: count
	// PromptPerception is the translation line of a checked prompt; args: perception, translation.
	PromptPerception string // in the learner's language, both args included
	StatsUpdated     string // stats.md header; args: date, mistakes, texts
	StatsEmpty       string // stats.md with no entries yet
}

// For picks the pack for the language; English is the fallback.
func For(language string) Pack {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "russian", "русский":
		return russian
	case "spanish", "español", "espanol":
		return spanish
	}
	return english
}

var english = Pack{
	CheckFailed:      "⚠ eng: prompt check failed, it sits in state/queue/failed — %s",
	WeekRate:         "📈 eng: last week — %.1f mistakes per 100 words in %d prompt words",
	WeekPrev:         " (week before: %.1f)",
	WeekTop:          "; most often: %s (%d)",
	DrillsRipe:       "🎯 eng: drills ripe — %s · /eng-drill %s · eng open drill/%s",
	CardsDue:         "🔁 eng: %d cards to review · /eng-drill review",
	SpeechPending:    "🎙 eng: %d speech recordings to review · eng speak review",
	NthTime:          ", %d-th time",
	MoreInDB:         " · %d more in the DB",
	PromptPerception: "🌐 eng: how it reads: %s · translation: %s",
	StatsUpdated:     "Updated: %s · mistakes: %d · texts: %d",
	StatsEmpty:       "The database is empty. The first review will fill it.",
}

var russian = Pack{
	CheckFailed:      "⚠ eng: проверка промпта не удалась, он лежит в state/queue/failed — %s",
	WeekRate:         "📈 eng: прошлая неделя — %.1f ошибки на 100 слов в %d словах промптов",
	WeekPrev:         " (неделей раньше %.1f)",
	WeekTop:          "; чаще всего: %s (%d)",
	DrillsRipe:       "🎯 eng: созрели дриллы — %s · /eng-drill %s · eng open drill/%s",
	CardsDue:         "🔁 eng: %d карточек к повторению · /eng-drill review",
	SpeechPending:    "🎙 eng: %d записей речи без разбора · eng speak review",
	NthTime:          ", %d-й раз",
	MoreInDB:         " · ещё %d в базе",
	PromptPerception: "🌐 eng: как это читается: %s · перевод: %s",
	StatsUpdated:     "Обновлено: %s · ошибок: %d · текстов: %d",
	StatsEmpty:       "База пуста. Первый разбор наполнит её.",
}

var spanish = Pack{
	CheckFailed:      "⚠ eng: falló la revisión del prompt, está en state/queue/failed — %s",
	WeekRate:         "📈 eng: semana pasada — %.1f errores por 100 palabras en %d palabras de prompts",
	WeekPrev:         " (semana anterior: %.1f)",
	WeekTop:          "; lo más frecuente: %s (%d)",
	DrillsRipe:       "🎯 eng: drills listos — %s · /eng-drill %s · eng open drill/%s",
	CardsDue:         "🔁 eng: %d tarjetas para repasar · /eng-drill review",
	SpeechPending:    "🎙 eng: %d grabaciones de voz sin revisar · eng speak review",
	NthTime:          ", %dª vez",
	MoreInDB:         " · %d más en la base",
	PromptPerception: "🌐 eng: cómo se lee: %s · traducción: %s",
	StatsUpdated:     "Actualizado: %s · errores: %d · textos: %d",
	StatsEmpty:       "La base está vacía. La primera revisión la llenará.",
}
