package woltserver

import (
	_ "embed"
	"fmt"
	"html/template"
	"net/http"

	"github.com/Masterminds/sprig"
	"github.com/gorilla/mux"
)

//go:embed minimal_join_html.template.html
var joinTemplate string

//go:embed details.template.json
var detailsTemplate string

//go:embed venue.template.json
var venueTemplate string

func (ws *WoltServer) extractID(r *http.Request) (string, error) {
	vars := mux.Vars(r)
	id, ok := vars["id"]
	if !ok {
		return "", fmt.Errorf("no id var in request")
	}
	return id, nil
}

func (ws *WoltServer) writeError(res http.ResponseWriter, status int, err error) {
	res.WriteHeader(status)
	_, _ = res.Write([]byte(fmt.Sprintf("Error: %v", err)))
}

func (ws *WoltServer) joinByShortIDHandler(res http.ResponseWriter, req *http.Request) {
	id, err := ws.extractID(req)
	if err != nil {
		ws.writeError(res, http.StatusBadRequest, err)
		return
	}

	order, ok, err := ws.getOrderByShortID(id)
	if err != nil {
		ws.writeError(res, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		ws.writeError(res, http.StatusBadRequest, ErrNoSuchOrder)
		return
	}

	htmlTmpl := template.Must(template.New("group").Funcs(sprig.HtmlFuncMap()).Parse(joinTemplate))
	if err = htmlTmpl.Execute(res, map[string]string{"ID": order.ID}); err != nil {
		ws.writeError(res, http.StatusInternalServerError, err)
		return
	}
}

func (ws *WoltServer) joinByIDHandler(res http.ResponseWriter, req *http.Request) {
	id, err := ws.extractID(req)
	if err != nil {
		ws.writeError(res, http.StatusBadRequest, err)
		return
	}

	_, ok := ws.getOrderByID(id)
	if !ok {
		ws.writeError(res, http.StatusBadRequest, ErrNoSuchOrder)
		return
	}
	res.WriteHeader(http.StatusOK)
}

func (ws *WoltServer) orderDetailsHandler(res http.ResponseWriter, req *http.Request) {
	id, err := ws.extractID(req)
	if err != nil {
		ws.writeError(res, http.StatusBadRequest, err)
		return
	}

	order, ok := ws.getOrderByID(id)
	if !ok {
		ws.writeError(res, http.StatusBadRequest, ErrNoSuchOrder)
		return
	}

	jsonTmpl := template.Must(template.New("details").Funcs(sprig.HtmlFuncMap()).Parse(detailsTemplate))
	if err = jsonTmpl.Execute(res, order); err != nil {
		ws.writeError(res, http.StatusInternalServerError, err)
		return
	}
}

func (ws *WoltServer) getVenueHandler(res http.ResponseWriter, req *http.Request) {
	id, err := ws.extractID(req)
	if err != nil {
		ws.writeError(res, http.StatusBadRequest, err)
		return
	}

	v, ok := ws.getVenue(id)
	if !ok {
		ws.writeError(res, http.StatusBadRequest, ErrNoSuchVenue)
		return
	}

	jsonTmpl := template.Must(template.New("venue").Funcs(sprig.HtmlFuncMap()).Parse(venueTemplate))
	if err = jsonTmpl.Execute(res, v); err != nil {
		ws.writeError(res, http.StatusInternalServerError, err)
		return
	}
}
