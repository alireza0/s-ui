package nodeagent

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

func HandlerV2(manager *ManagerV2) http.Handler {
	mux := http.NewServeMux()
	status := func(w http.ResponseWriter, r *http.Request) { statusV2(manager, w, r) }
	mux.HandleFunc("/v2/status", status)
	mux.HandleFunc("/v2/certificate", func(w http.ResponseWriter, r *http.Request) { certificateV2(manager, w, r) })
	mux.HandleFunc("/v2/access", func(w http.ResponseWriter, r *http.Request) { writeStateV2(manager, w, r, true) })
	mux.HandleFunc("/v2/spec", func(w http.ResponseWriter, r *http.Request) { writeStateV2(manager, w, r, false) })
	return mux
}

func statusV2(manager *ManagerV2, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, methodNotAllowed, 405)
		return
	}
	w.Header().Set(contentTypeHeader, jsonContentType)
	status := manager.Status()
	if runtime, ok := manager.runtime.(interface{ Traffic() *TrafficSample }); ok {
		status.Traffic = runtime.Traffic()
	}
	if accounting, ok := manager.runtime.(interface{ TrafficAccountingError() string }); ok && status.Error == "" {
		status.Error = accounting.TrafficAccountingError()
	}
	_ = json.NewEncoder(w).Encode(status)
}

func certificateV2(manager *ManagerV2, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, methodNotAllowed, 405)
		return
	}
	if manager.resources == nil {
		http.Error(w, "certificate storage unavailable", 503)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 3<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request CertificateRequest
	if decoder.Decode(&request) != nil || decoder.Decode(new(any)) != io.EOF {
		http.Error(w, "invalid certificate body", 400)
		return
	}
	if err := manager.resources.Configure(request); err != nil {
		code := 422
		if errors.Is(err, ErrConflict) {
			code = 409
		}
		http.Error(w, "certificate request not accepted", code)
		return
	}
	w.Header().Set(contentTypeHeader, jsonContentType)
	json.NewEncoder(w).Encode(manager.Status())
}

func writeStateV2(manager *ManagerV2, w http.ResponseWriter, r *http.Request, access bool) {
	if r.Method != http.MethodPut {
		http.Error(w, methodNotAllowed, 405)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	var err error
	if access {
		var value AccessSnapshot
		if d.Decode(&value) != nil || d.Decode(new(any)) != io.EOF {
			http.Error(w, "invalid access body", 400)
			return
		}
		err = manager.ApplyAccess(value)
	} else {
		var value Structure
		if d.Decode(&value) != nil || d.Decode(new(any)) != io.EOF {
			http.Error(w, "invalid spec body", 400)
			return
		}
		err = manager.ApplyStructure(value)
	}
	if err != nil {
		if errors.Is(err, ErrConflict) {
			http.Error(w, "identity or revision conflict", 409)
		} else {
			http.Error(w, "state was not applied", 422)
		}
		return
	}
	w.Header().Set(contentTypeHeader, jsonContentType)
	_ = json.NewEncoder(w).Encode(manager.Status())
}
