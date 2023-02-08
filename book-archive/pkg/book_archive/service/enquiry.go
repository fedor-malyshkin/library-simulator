package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fedor-malyshkin/library-simulator/book-archive/pkg/book_archive"
	"github.com/fedor-malyshkin/library-simulator/book-archive/pkg/book_archive/config"
	"github.com/fedor-malyshkin/library-simulator/common/pkg/svclog"
	"github.com/rs/zerolog"
	"io"
	"net/http"
	"time"
)

type EnquiryHandler struct {
	log   zerolog.Logger
	dbHnd *Handler
}

func NewEnquiryHandler(cfg *config.Config,
	appCtx *book_archive.AppContext,
	dbHnd *Handler) *EnquiryHandler {

	return &EnquiryHandler{
		log:   svclog.Service(appCtx.Logger, "enquiry-handler"),
		dbHnd: dbHnd,
	}
}

func (h EnquiryHandler) ProcessEnquiry(query string) (int, string) {
	var res string
	res, ok := h.dbHnd.getFromCache(query)
	if ok {
		return http.StatusOK, res
	}
	res, err := h.queryArchive(query)
	if err != nil {
		h.log.Err(err).Msg("error requesting archive")
		return http.StatusInternalServerError, ""
	}
	// update cache
	h.dbHnd.cacheCh <- CacheRec{
		Query: query,
		Data:  res,
	}
	return http.StatusOK, res
}

func (h EnquiryHandler) queryArchive(query string) (string, error) {
	h.log.Info().Str("query", query).Msg("requesting GitHub")
	//------------
	start := time.Now()
	//------------
	body, err := h.queryGitHub(query)
	if err != nil {
		return "", errors.New("error requesting GitHub")
	}
	//------------
	h.log.Debug().Dur("dur (ms)", time.Now().Sub(start)).Msg("requesting GitHub")
	start = time.Now()
	//------------
	result, err := h.formatResponse(body)
	if err != nil {
		return "", errors.New("error forming response")
	}
	//------------
	h.log.Debug().Dur("dur (ms)", time.Now().Sub(start)).Msg("parsing/forming GitHub")
	//------------
	return result, nil
}

func (h EnquiryHandler) queryGitHub(query string) ([]byte, error) {
	req := fmt.Sprintf("https://api.github.com/search/repositories?q=%s", query)
	resp, err := http.Get(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (h EnquiryHandler) formatResponse(data []byte) (string, error) {
	type RepoItem struct {
		Description string `json:"description"`
		Url         string `json:"url"`
	}

	type ReposResp struct {
		Items []RepoItem `json:"items"`
	}

	var resp ReposResp
	err := json.Unmarshal(data, &resp)
	if err != nil {
		return "", err
	}
	var result []string
	for i, e := range resp.Items {
		if i > 3 {
			break
		}
		result = append(result, fmt.Sprintf("%s - %s", e.Description, e.Url))
	}

	if b, err := json.Marshal(result); err != nil {
		return "", err
	} else {
		return string(b), nil
	}
}
