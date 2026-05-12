package render

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
)

func JSON(w http.ResponseWriter, status int, body any) {
	data, err := json.Marshal(body)
	if err != nil {
		log.Error().Err(err).Msg("failed to encode body")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if _, err := w.Write(data); err != nil {
		log.Error().
			Err(err).
			Msg("failed to write response")
	}
}
