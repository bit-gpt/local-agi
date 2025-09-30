package utils

import (
	"strings"
	"time"

	"google.golang.org/api/calendar/v3"
)

type EventTime struct {
	start time.Time
	end   time.Time
}

type TitleMatch struct {
	Exact   bool
	Similar bool
}

type Event struct {
	ID       string     `json:"id"`
	Summary  string     `json:"summary"`
	Location string     `json:"location"`
	Status   string     `json:"status"`
	Start    *EventTime `json:"start,omitempty"`
}

func CalculateEventSimilarity(event1, event2 *calendar.Event) float64 {
	event1IsAllDay := IsAllDayEvent(event1)
	event2IsAllDay := IsAllDayEvent(event2)

	if event1IsAllDay != event2IsAllDay {
		return 0.2
	}

	titleMatch := TitlesMatch(event1.Summary, event2.Summary)
	timeOverlap := EventsOverlap(event1, event2)
	sameDay := EventsOnSameDay(event1, event2)

	if titleMatch.Exact && timeOverlap {
		return 0.95
	}

	if titleMatch.Similar && timeOverlap {
		return 0.7
	}

	if titleMatch.Exact && sameDay {
		return 0.6
	}

	if titleMatch.Exact && !sameDay {
		return 0.4
	}

	if titleMatch.Similar {
		return 0.3
	}

	return 0.1
}

func IsAllDayEvent(event *calendar.Event) bool {
	if event.Start == nil {
		return false
	}
	return event.Start.DateTime == "" && event.Start.Date != ""
}

func TitlesMatch(title1, title2 string) TitleMatch {
	if title1 == "" || title2 == "" {
		return TitleMatch{Exact: false, Similar: false}
	}

	t1 := strings.ToLower(strings.TrimSpace(title1))
	t2 := strings.ToLower(strings.TrimSpace(title2))

	if t1 == t2 {
		return TitleMatch{Exact: true, Similar: true}
	}

	if strings.Contains(t1, t2) || strings.Contains(t2, t1) {
		return TitleMatch{Exact: false, Similar: true}
	}

	words1 := getSignificantWords(t1)
	words2 := getSignificantWords(t2)

	if len(words1) > 0 && len(words2) > 0 {
		commonWords := countCommonWords(words1, words2)
		minWords := min(len(words1), len(words2))
		similarity := float64(commonWords) / float64(minWords)

		return TitleMatch{Exact: false, Similar: similarity >= 0.5}
	}

	return TitleMatch{Exact: false, Similar: false}
}

func getSignificantWords(text string) []string {
	words := strings.Fields(text)
	var significant []string

	for _, word := range words {
		if len(word) > 3 {
			significant = append(significant, word)
		}
	}

	return significant
}

func countCommonWords(words1, words2 []string) int {
	wordSet := make(map[string]bool)
	for _, word := range words2 {
		wordSet[word] = true
	}

	count := 0
	for _, word := range words1 {
		if wordSet[word] {
			count++
		}
	}

	return count
}

func EventsOnSameDay(event1, event2 *calendar.Event) bool {
	time1 := getEventTime(event1)
	time2 := getEventTime(event2)

	if time1 == nil || time2 == nil {
		return false
	}

	// Compare dates only (ignore time)
	y1, m1, d1 := time1.start.Date()
	y2, m2, d2 := time2.start.Date()

	return y1 == y2 && m1 == m2 && d1 == d2
}

func getEventTime(event *calendar.Event) *EventTime {
	if event.Start == nil || event.End == nil {
		return nil
	}

	startTime := event.Start.DateTime
	if startTime == "" {
		startTime = event.Start.Date
	}

	endTime := event.End.DateTime
	if endTime == "" {
		endTime = event.End.Date
	}

	if startTime == "" || endTime == "" {
		return nil
	}

	start, err1 := parseEventTime(startTime)
	end, err2 := parseEventTime(endTime)

	if err1 != nil || err2 != nil {
		return nil
	}

	return &EventTime{
		start: start,
		end:   end,
	}
}

func parseEventTime(timeStr string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, timeStr)
	if err == nil {
		return t, nil
	}

	// Try date-only format
	t, err = time.Parse("2006-01-02", timeStr)
	if err == nil {
		return t, nil
	}

	return time.Time{}, err
}

func EventsOverlap(event1, event2 *calendar.Event) bool {
	time1 := getEventTime(event1)
	time2 := getEventTime(event2)

	if time1 == nil || time2 == nil {
		return false
	}

	return time1.start.Before(time2.end) && time2.start.Before(time1.end)
}

func CalculateOverlapDuration(event1, event2 *calendar.Event) int64 {
	time1 := getEventTime(event1)
	time2 := getEventTime(event2)

	if time1 == nil || time2 == nil {
		return 0
	}

	overlapStart := maxTime(time1.start, time2.start)
	overlapEnd := minTime(time1.end, time2.end)

	duration := overlapEnd.Sub(overlapStart).Milliseconds()
	if duration < 0 {
		return 0
	}

	return duration
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxTime(t1, t2 time.Time) time.Time {
	if t1.After(t2) {
		return t1
	}
	return t2
}

func minTime(t1, t2 time.Time) time.Time {
	if t1.Before(t2) {
		return t1
	}
	return t2
}
