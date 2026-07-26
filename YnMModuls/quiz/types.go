// YnMModuls/quiz/types.go
package quiz

import (
    "sync"
    "time"
)

type Question struct {
    ID       int      `json:"id"`
    Text     string   `json:"text"`
    Options  []string `json:"options"`
    Correct  int      `json:"correct"`
    Category string   `json:"category"`
}

type PlayerAnswer struct {
    Nick      string
    Answer    int
    IsCorrect bool
    Time      time.Time
}

type Game struct {
    Channel    string
    Questions  []Question
    CurrentQ   int
    Active     bool
    Answers    map[string]PlayerAnswer
    Timer      *time.Timer
    Mu         sync.Mutex
    StartTime  time.Time
    Scores     map[string]int
}

type QuizService struct {
    Games      map[string]*Game
    Questions  []Question
    Mu         sync.RWMutex
    Bot        interface{} // YnM bot példány
}