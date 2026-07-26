// YnMModuls/quiz/questions.go
package quiz

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
)

func LoadQuestions(dataDir string) ([]Question, error) {
    jsonPath := filepath.Join(dataDir, "questions.json")
    if data, err := os.ReadFile(jsonPath); err == nil {
        var questions []Question
        if err := json.Unmarshal(data, &questions); err == nil {
            return questions, nil
        }
        fmt.Printf("JSON dekódolási hiba: %v\n", err)
    }
    
    return getDefaultQuestions(), nil
}

func getDefaultQuestions() []Question {
    return []Question{
        {
            ID:       1,
            Text:     "Mi a fővárosa Magyarországnak?",
            Options:  []string{"Bécs", "Budapest", "Prága", "Bratislava"},
            Correct:  1,
            Category: "Földrajz",
        },
        {
            ID:       2,
            Text:     "Melyik évben volt a honfoglalás?",
            Options:  []string{"800", "896", "955", "1000"},
            Correct:  1,
            Category: "Történelem",
        },
        {
            ID:       3,
            Text:     "Melyik a legnagyobb tó Magyarországon?",
            Options:  []string{"Balaton", "Velencei-tó", "Fertő-tó", "Tisza-tó"},
            Correct:  0,
            Category: "Földrajz",
        },
        {
            ID:       4,
            Text:     "Ki volt az első magyar király?",
            Options:  []string{"Koppány", "Géza", "Szent István", "Árpád"},
            Correct:  2,
            Category: "Történelem",
        },
        {
            ID:       5,
            Text:     "Melyik városban található a Parlament?",
            Options:  []string{"Debrecen", "Szeged", "Budapest", "Pécs"},
            Correct:  2,
            Category: "Földrajz",
        },
        {
            ID:       6,
            Text:     "Hány megyéje van Magyarországnak?",
            Options:  []string{"17", "19", "20", "23"},
            Correct:  1,
            Category: "Földrajz",
        },
        {
            ID:       7,
            Text:     "Melyik a leghosszabb folyó Magyarországon?",
            Options:  []string{"Duna", "Tisza", "Rába", "Dráva"},
            Correct:  1,
            Category: "Földrajz",
        },
        {
            ID:       8,
            Text:     "Mikor lett Magyarország az EU tagja?",
            Options:  []string{"2002", "2004", "2006", "2008"},
            Correct:  1,
            Category: "Történelem",
        },
    }
}