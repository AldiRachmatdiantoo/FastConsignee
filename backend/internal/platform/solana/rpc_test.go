package solana_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/testkit"
)

func rpcServer(t *testing.T, reply string, inspect func(params []json.RawMessage)) *solana.RPC {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if inspect != nil {
			inspect(req.Params)
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(reply)); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	return solana.NewRPC(server.URL)
}

// Transaksi versi baru yang memuat reference tidak boleh menahan order selamanya.
func TestUnsupportedTransactionVersionIsMalformed(t *testing.T) {
	client := rpcServer(t, `{"jsonrpc":"2.0","id":1,"error":{"code":-32015,"message":"Transaction version (2) is not supported"}}`, nil)
	_, err := client.GetTransaction(context.Background(), testkit.NewSignature())
	if !errors.Is(err, solana.ErrMalformed) {
		t.Fatalf("seharusnya ErrMalformed: %v", err)
	}
}

func TestGetTransactionAsksForNewestVersion(t *testing.T) {
	var opts struct {
		Version *uint64 `json:"maxSupportedTransactionVersion"`
	}
	client := rpcServer(t, `{"jsonrpc":"2.0","id":1,"result":null}`, func(params []json.RawMessage) {
		if err := json.Unmarshal(params[1], &opts); err != nil {
			t.Error(err)
		}
	})
	_, err := client.GetTransaction(context.Background(), testkit.NewSignature())
	if !errors.Is(err, solana.ErrNotFound) {
		t.Fatalf("seharusnya ErrNotFound: %v", err)
	}
	if opts.Version == nil || *opts.Version != 1 {
		t.Fatalf("versi %v", opts.Version)
	}
}

func TestOtherRPCErrorsStayRetryable(t *testing.T) {
	client := rpcServer(t, `{"jsonrpc":"2.0","id":1,"error":{"code":-32005,"message":"Node is behind"}}`, nil)
	_, err := client.GetTransaction(context.Background(), testkit.NewSignature())
	if err == nil || errors.Is(err, solana.ErrMalformed) || errors.Is(err, solana.ErrNotFound) {
		t.Fatalf("gangguan RPC harus dicoba lagi: %v", err)
	}
}
