package YnM

import (
    "encoding/json"
    "net/http"
)

// GET /tools
func HandleGetTools(w http.ResponseWriter, r *http.Request) {
    tools := []map[string]interface{}{
        {
            "name":        "get_data",
            "description": "Fetch data from your system",
            "inputSchema": map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "id": map[string]interface{}{"type": "string"},
                },
                "required": []string{"id"},
            },
        },
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(tools)
}

// POST /tools/{toolName}
func HandleToolCall(w http.ResponseWriter, r *http.Request) {
    var args map[string]interface{}
    json.NewDecoder(r.Body).Decode(&args)

    result := map[string]interface{}{
        "status": "ok",
        "data":   "Ez egy példa output a get_data toolból",
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(result)
}
