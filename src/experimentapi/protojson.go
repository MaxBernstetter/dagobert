package experimentapi

import (
	"io"
	"net/http"

	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var (
	marshalOpts = protojson.MarshalOptions{
		UseProtoNames:   false, // camelCase for REST
		EmitUnpopulated: true,
	}
	unmarshalOpts = protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
)

func writeProto(w http.ResponseWriter, code int, msg proto.Message) {
	b, err := marshalOpts.Marshal(msg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(b)
}

func writeProtoBinary(w http.ResponseWriter, code int, msg proto.Message) {
	b, err := proto.Marshal(msg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/protobuf")
	w.WriteHeader(code)
	_, _ = w.Write(b)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeProto(w, code, &v1.ErrorResponse{Error: msg})
}

func readProto(r *http.Request, msg proto.Message) error {
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if len(b) == 0 {
		return nil
	}
	return unmarshalOpts.Unmarshal(b, msg)
}
