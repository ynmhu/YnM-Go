// YnMModuls/quiz/quiz.go
package quiz

import (
    "fmt"
    "strings"
    "time"
)

// Modul neve
const ModuleName = "quiz"

// BotInterface - a YnM bot minimális interfésze, amire a modulnak szüksége van
type BotInterface interface {
    Privmsg(channel, message string)
    // Ha más függvények is kellenek, itt bővíthető
}

// QuizModule struktúra
type QuizModule struct {
    service *QuizService
    bot     BotInterface
    config  map[string]interface{}
    timeout time.Duration
}

// Új modul létrehozása
func New(bot BotInterface, config map[string]interface{}) *QuizModule {
    // Kérdések betöltése
    questions, _ := LoadQuestions("./data")
    
    timeout := 5 // alapértelmezett másodperc
    if t, ok := config["timeout"].(int); ok && t > 0 {
        timeout = t
    }
    
    return &QuizModule{
        service: NewQuizService(questions),
        bot:     bot,
        config:  config,
        timeout: time.Duration(timeout) * time.Second,
    }
}

// Modul inicializálása
func (m *QuizModule) Init() error {
    // Itt lehetne regisztrálni a parancsokat, de mivel nem ismerjük a YnM API-t,
    // ezt később a main.go-ban fogjuk megtenni
    
    // Takarítási ciklus indítása
    go m.cleanupLoop()
    
    return nil
}

// !quiz parancs kezelése (ezt hívja majd a main.go)
func (m *QuizModule) HandleQuizCommand(channel, nick, message string) {
    parts := strings.Fields(message)
    
    if len(parts) == 1 {
        success, msg := m.service.StartGame(channel)
        m.sendMessage(channel, msg)
        
        if success {
            m.sendNextQuestion(channel)
        }
        return
    }
    
    if len(parts) == 2 {
        response, _ := m.service.HandleAnswer(channel, nick, parts[1])
        m.sendMessage(channel, response)
        return
    }
}

// !top parancs
func (m *QuizModule) HandleTopCommand(channel, nick, message string) {
    m.sendMessage(channel, "🏆 Toplista funkció még fejlesztés alatt!")
}

// Üzenet küldése
func (m *QuizModule) sendMessage(channel, msg string) {
    if m.bot != nil {
        m.bot.Privmsg(channel, msg)
    } else {
        fmt.Printf("[%s] %s\n", channel, msg)
    }
}

// Következő kérdés
func (m *QuizModule) sendNextQuestion(channel string) {
    game, exists := m.service.GetGame(channel)
    if !exists || !game.Active {
        return
    }
    
    game.Mu.Lock()
    defer game.Mu.Unlock()
    
    if game.CurrentQ >= len(game.Questions) {
        go m.endGame(channel)
        return
    }
    
    q := game.Questions[game.CurrentQ]
    game.Answers = make(map[string]PlayerAnswer)
    
    // Kérdés kiírása
    m.sendMessage(channel, fmt.Sprintf("📝 %d. kérdés: %s", game.CurrentQ+1, q.Text))
    
    var opts []string
    for i, opt := range q.Options {
        opts = append(opts, fmt.Sprintf("%d) %s", i+1, opt))
    }
    m.sendMessage(channel, "Választható: "+strings.Join(opts, " | "))
    
    // Időzítő indítása
    if game.Timer != nil {
        game.Timer.Stop()
    }
    game.Timer = time.AfterFunc(m.timeout, func() {
        m.timeUp(channel)
    })
}

// Idő lejárt
func (m *QuizModule) timeUp(channel string) {
    game, exists := m.service.GetGame(channel)
    if !exists || !game.Active {
        return
    }
    
    game.Mu.Lock()
    defer game.Mu.Unlock()
    
    if game.CurrentQ >= len(game.Questions) {
        return
    }
    
    q := game.Questions[game.CurrentQ]
    
    m.sendMessage(channel, fmt.Sprintf("⏰ Idő! Helyes válasz: %d) %s",
        q.Correct+1, q.Options[q.Correct]))
    
    var correctPlayers []string
    for nick, answer := range game.Answers {
        if answer.IsCorrect {
            correctPlayers = append(correctPlayers, nick)
        }
    }
    
    if len(correctPlayers) > 0 {
        m.sendMessage(channel, fmt.Sprintf("✅ Helyes válasz: %s",
            strings.Join(correctPlayers, ", ")))
    } else {
        m.sendMessage(channel, "❌ Senki nem válaszolt helyesen!")
    }
    
    game.CurrentQ++
    if game.CurrentQ >= len(game.Questions) {
        go m.endGame(channel)
    } else {
        go m.sendNextQuestion(channel)
    }
}

// Játék vége
func (m *QuizModule) endGame(channel string) {
    game, exists := m.service.GetGame(channel)
    if !exists {
        return
    }
    
    game.Mu.Lock()
    defer game.Mu.Unlock()
    
    game.Active = false
    
    m.sendMessage(channel, "🏁 Kvíz vége! Végeredmény:")
    
    if len(game.Scores) == 0 {
        m.sendMessage(channel, "❌ Senki nem szerzett pontot!")
    } else {
        for nick, score := range game.Scores {
            m.sendMessage(channel, fmt.Sprintf("  %s: %d pont", nick, score))
        }
    }
    
    // Játék törlése
    m.service.ClearGame(channel)
}

// Takarítási ciklus
func (m *QuizModule) cleanupLoop() {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()
    
    for range ticker.C {
        // Itt lehetne ellenőrizni a régi játékokat
    }
}