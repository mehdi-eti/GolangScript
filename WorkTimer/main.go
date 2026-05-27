package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

const logFileName = "work_log.json"

type BreakInterval struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	Duration int64  `json:"duration_seconds"`
}

type WorkLog struct {
	Date              string          `json:"date"`
	SessionStart      string          `json:"session_start"`
	SessionEnd        string          `json:"session_end"`
	TotalWorkSeconds  int64           `json:"total_work_seconds"`
	TotalBreakSeconds int64           `json:"total_break_seconds"`
	Breaks            []BreakInterval `json:"breaks"`
}

type AppState struct {
	mu sync.Mutex

	mainSeconds int64
	subSeconds  int64

	breakSeconds int64

	running   bool
	onBreak   bool
	breakFrom time.Time

	sessionStart time.Time
	sessionEnd   time.Time

	breaks []BreakInterval
}

var state = &AppState{}

func main() {
	a := app.New()
	w := a.NewWindow("Work Timer")

	w.Resize(fyne.NewSize(500, 400))

	mainTimerLabel := widget.NewLabelWithStyle(
		formatDuration(0),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	subTimerLabel := widget.NewLabelWithStyle(
		formatDuration(0),
		fyne.TextAlignCenter,
		fyne.TextStyle{},
	)

	mainTimerLabel.TextStyle = fyne.TextStyle{Bold: true}

	reportOutput := widget.NewMultiLineEntry()
	reportOutput.Wrapping = fyne.TextWrapWord
	reportOutput.Disable()

	go runTicker(mainTimerLabel, subTimerLabel)

	startBtn := widget.NewButton("Start / Resume", func() {
		state.mu.Lock()
		defer state.mu.Unlock()

		if state.sessionStart.IsZero() {
			state.sessionStart = time.Now()
		}

		state.running = true
		state.onBreak = false
	})

	startBreakBtn := widget.NewButton("Start Break", func() {
		state.mu.Lock()
		defer state.mu.Unlock()

		if !state.running || state.onBreak {
			return
		}

		state.running = false
		state.onBreak = true
		state.breakFrom = time.Now()
	})

	endBreakBtn := widget.NewButton("End Break", func() {
		state.mu.Lock()
		defer state.mu.Unlock()

		if !state.onBreak {
			return
		}

		breakEnd := time.Now()
		duration := int64(breakEnd.Sub(state.breakFrom).Seconds())

		state.breakSeconds += duration

		state.breaks = append(state.breaks, BreakInterval{
			Start:    state.breakFrom.Format(time.RFC3339),
			End:      breakEnd.Format(time.RFC3339),
			Duration: duration,
		})

		state.running = true
		state.onBreak = false
	})

	resetSubBtn := widget.NewButton("Reset Sub-timer", func() {
		state.mu.Lock()
		defer state.mu.Unlock()

		state.subSeconds = 0
	})

	endDayBtn := widget.NewButton("End Day", func() {
		state.mu.Lock()

		state.running = false
		state.onBreak = false
		state.sessionEnd = time.Now()

		entry := WorkLog{
			Date:              time.Now().Format("2006-01-02"),
			SessionStart:      state.sessionStart.Format(time.RFC3339),
			SessionEnd:        state.sessionEnd.Format(time.RFC3339),
			TotalWorkSeconds:  state.mainSeconds,
			TotalBreakSeconds: state.breakSeconds,
			Breaks:            state.breaks,
		}

		state.mu.Unlock()

		err := saveWorkLog(entry)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}

		summary := fmt.Sprintf(
			"Day Saved\n\nWork: %s\nBreak: %s\nBreak Count: %d",
			formatDuration(entry.TotalWorkSeconds),
			formatDuration(entry.TotalBreakSeconds),
			len(entry.Breaks),
		)

		dialog.ShowInformation("Summary", summary, w)

		resetState()
	})

	reportBtn := widget.NewButton("View Report", func() {
		report, err := generateReport()
		if err != nil {
			dialog.ShowError(err, w)
			return
		}

		reportOutput.SetText(report)

		err = os.WriteFile("report.txt", []byte(report), 0644)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
	})

	buttons := container.NewVBox(
		startBtn,
		startBreakBtn,
		endBreakBtn,
		resetSubBtn,
		endDayBtn,
		reportBtn,
	)

	content := container.NewVBox(
		layout.NewSpacer(),
		mainTimerLabel,
		subTimerLabel,
		layout.NewSpacer(),
		buttons,
		widget.NewLabel("Report"),
		reportOutput,
	)

	w.SetContent(content)
	w.ShowAndRun()
}

func runTicker(mainLabel, subLabel *widget.Label) {
	ticker := time.NewTicker(time.Second)

	for range ticker.C {
		state.mu.Lock()

		if state.running {
			state.mainSeconds++
			state.subSeconds++
		}

		mainText := formatDuration(state.mainSeconds)
		subText := formatDuration(state.subSeconds)

		state.mu.Unlock()

		mainLabel.SetText(mainText)
		subLabel.SetText(subText)
	}
}

func resetState() {
	state.mu.Lock()
	defer state.mu.Unlock()

	state.mainSeconds = 0
	state.subSeconds = 0
	state.breakSeconds = 0

	state.running = false
	state.onBreak = false

	state.sessionStart = time.Time{}
	state.sessionEnd = time.Time{}

	state.breaks = nil
}

func formatDuration(seconds int64) string {
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60

	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func saveWorkLog(entry WorkLog) error {
	logs, err := loadLogs()
	if err != nil {
		return err
	}

	found := false

	for i := range logs {
		if logs[i].Date == entry.Date {
			logs[i].TotalWorkSeconds += entry.TotalWorkSeconds
			logs[i].TotalBreakSeconds += entry.TotalBreakSeconds
			logs[i].Breaks = append(logs[i].Breaks, entry.Breaks...)
			logs[i].SessionEnd = entry.SessionEnd

			if logs[i].SessionStart == "" {
				logs[i].SessionStart = entry.SessionStart
			}

			found = true
			break
		}
	}

	if !found {
		logs = append(logs, entry)
	}

	data, err := json.MarshalIndent(logs, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(logFileName, data, 0644)
}

func loadLogs() ([]WorkLog, error) {
	path := filepath.Clean(logFileName)

	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return []WorkLog{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var logs []WorkLog

	if len(data) == 0 {
		return []WorkLog{}, nil
	}

	err = json.Unmarshal(data, &logs)
	if err != nil {
		return nil, err
	}

	return logs, nil
}

func generateReport() (string, error) {
	logs, err := loadLogs()
	if err != nil {
		return "", err
	}

	report := "DATE | WORK | BREAK | BREAK COUNT\n"
	report += "----------------------------------------\n"

	for _, log := range logs {
		line := fmt.Sprintf(
			"%s | %s | %s | %d\n",
			log.Date,
			formatDuration(log.TotalWorkSeconds),
			formatDuration(log.TotalBreakSeconds),
			len(log.Breaks),
		)

		report += line
	}

	return report, nil
}
