package enroll

import (
	"context"
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/fullsailor/pkcs7"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/groob/plist"
)

type HTTPHandlers struct {
	EnrollHandler    http.Handler
	OTAEnrollHandler http.Handler

	// In Apple's Over-the-Air design Phases 2 and 3 happen over the same URL.
	// The differentiator is which certificate signed the CMS POST body.
	OTAPhase2Phase3Handler http.Handler
}

func MakeHTTPHandlers(ctx context.Context, endpoints Endpoints, opts ...httptransport.ServerOption) HTTPHandlers {
	h := HTTPHandlers{
		EnrollHandler: httptransport.NewServer(
			endpoints.GetEnrollEndpoint,
			decodeMDMEnrollRequest,
			encodeResponse,
			opts...,
		),
		OTAEnrollHandler: httptransport.NewServer(
			endpoints.OTAEnrollEndpoint,
			nilRequest,
			encodeResponse,
			opts...,
		),
		OTAPhase2Phase3Handler: httptransport.NewServer(
			endpoints.OTAPhase2Phase3Endpoint,
			decodeOTAPhase2Phase3Request,
			encodeResponse,
			opts...,
		),
	}
	return h
}

func nilRequest(_ context.Context, _ *http.Request) (interface{}, error) {
	return nil, nil
}

func decodeMDMEnrollRequest(_ context.Context, r *http.Request) (interface{}, error) {
	switch r.Method {
	case "GET":
		return mdmEnrollRequest{}, nil
	case "POST":
		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		p7, err := pkcs7.Parse(data)
		if err != nil {
			return nil, err
		}
		// TODO: We should verify but not currently possible. Apple
		// does no provide a cert for the CA.
		var request depEnrollmentRequest
		if err := plist.Unmarshal(p7.Content, &request); err != nil {
			return nil, err
		}
		return request, nil
	default:
		return nil, errors.New("unknown enrollment method")
	}
}

func encodeResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	w.Header().Set("Content-Type", "application/x-apple-aspen-config")

	enc := plist.NewEncoder(w)
	enc.Indent("  ")
	if err := enc.Encode(response); err != nil {
		return err
	}

	return nil
}

func decodeOTAPhase2Phase3Request(_ context.Context, r *http.Request) (interface{}, error) {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	p7, err := pkcs7.Parse(data)
	if err != nil {
		return nil, err
	}
	err = p7.Verify()
	if err != nil {
		return nil, err
	}
	var request otaEnrollmentRequest
	err = plist.Unmarshal(p7.Content, &request)
	if err != nil {
		return nil, err
	}
	return mdmOTAPhase2Phase3Request{request, p7}, nil
}