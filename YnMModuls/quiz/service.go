// YnMModuls/quiz/service.go
package quiz

import (
    "fmt"
    "math/rand"
    "time"
)

func NewQuizService(questions []Question) *QuizService {
    return &QuizService{
        Games:     make(map[string]*Game),
        Questions: questions,
    }
}

func (qs *QuizService) StartGame(channel string) (bool, string) {
    qs.Mu.Lock()
    defer qs.Mu.Unlock()
    
    if game, exists := qs.Games[channel]; exists && game.Active {
        return false, "Már van aktív kvíz ezen a csatornán!"
    }
    
    questions := qs.getRandomQuestions(4)
    if len(questions) < 4 {
        return false, "Nincs elég kérdés az adatbázisban!"
    }
    
    game := &Game{
        Channel:   channel,
        Questions: questions,
        CurrentQ:  0,
        Active:    true,
        Answers:   make(map[string]PlayerAnswer),
        Scores:    make(map[string]int),
        StartTime: time.Now(),
    }
    
    qs.Games[channel] = game
    return true, "🎯 Kvíz indul! 4 kérdés, 5 másodperc válaszidő!"
}

func (qs *QuizService) HandleAnswer(channel, nick, answerStr string) (string, bool) {
    qs.Mu.RLock()
    game, exists := qs.Games[channel]
    qs.Mu.RUnlock()
    
    if !exists || !game.Active {
        return "Nincs aktív kvíz! Használd: !quiz", false
    }
    
    game.Mu.Lock()
    defer game.Mu.Unlock()
    
    if _, answered := game.Answers[nick]; answered {
        return "Már válaszoltál erre a kérdésre!", false
    }
    
    answerIdx := -1
    switch answerStr {
    case "1", "2", "3", "4":
        answerIdx = int(answerStr[0] - '1')
    default:
        return "Érvénytelen válasz! Használd: 1, 2, 3 vagy 4", false
    }
    
    q := game.Questions[game.CurrentQ]
    correct := answerIdx == q.Correct
    
    game.Answers[nick] = PlayerAnswer{
        Nick:      nick,
        Answer:    answerIdx,
        IsCorrect: correct,
        Time:      time.Now(),
    }
    
    if correct {
        game.Scores[nick]++
        return fmt.Sprintf("✅ %s helyes válasz! (+1 pont)", nick), true
    }
    
    return fmt.Sprintf("❌ %s rossz válasz. Helyes: %d) %s",
        nick, q.Correct+1, q.Options[q.Correct]), false
}

func (qs *QuizService) getRandomQuestions(count int) []Question {
    if len(qs.Questions) < count {
        return qs.Questions
    }
    
    questions := make([]Question, len(qs.Questions))
    copy(questions, qs.Questions)
    rand.Shuffle(len(questions), func(i, j int) {
        questions[i], questions[j] = questions[j], questions[i]
    })
    
    return questions[:count]
}

func (qs *QuizService) GetGame(channel string) (*Game, bool) {
    qs.Mu.RLock()
    defer qs.Mu.RUnlock()
    game, exists := qs.Games[channel]
    return game, exists
}

func (qs *QuizService) DeleteGame(channel string) {
    qs.Mu.Lock()
    defer qs.Mu.Unlock()
    delete(qs.Games, channel)
}

func (qs *QuizService) ClearGame(channel string) {
    qs.Mu.Lock()
    defer qs.Mu.Unlock()
    if game, exists := qs.Games[channel]; exists {
        game.Active = false
        if game.Timer != nil {
            game.Timer.Stop()
        }
    }
    delete(qs.Games, channel)
}