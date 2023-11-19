package restconf

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/parser"
)

type handlerImpl http.HandlerFunc

func (impl handlerImpl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	impl(w, r)
}

func TestForm(t *testing.T) {
	if os.Getenv("TRAVIS") == "true" {
		// no web servers allowed in CI
		t.Skip()
		return
	}

	m, err := parser.LoadModuleFromString(nil, `
		module test {
			rpc x {
				input {
					leaf a {
						type string;
					}
					anydata b;					
				}
			}
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan bool, 1)
	handler := func(w http.ResponseWriter, r *http.Request) {
		b := node.NewBrowser(m, formDummyNode(t))
		x := m.Actions()["x"]
		input, err := readInput(Strict, YangDataJsonMimeType1, r, x)
		chkErr(t, err)
		xsel, err := b.Root().Find("x")
		chkErr(t, err)
		_, err = xsel.Action(input)
		chkErr(t, err)
		w.Write([]byte("ok"))
		t.Log("form received")
		done <- true
	}
	srv := &http.Server{Addr: "127.0.0.1:9999", Handler: handlerImpl(handler)}
	go srv.ListenAndServe()
	defer srv.Shutdown(context.TODO())
	// wait for server to start
	<-time.After(10 * time.Millisecond)

	var buf bytes.Buffer
	form := multipart.NewWriter(&buf)
	dataPart, err := form.CreateFormField("a")
	chkErr(t, err)
	fmt.Fprint(dataPart, "hello")
	filePart, err := form.CreateFormFile("b", "b")
	chkErr(t, err)
	fmt.Fprint(filePart, "hello world")
	chkErr(t, form.Close())
	req, err := http.NewRequest("POST", "http://"+srv.Addr, &buf)
	chkErr(t, err)
	req.Header.Set("Content-Type", form.FormDataContentType())
	_, err = http.DefaultClient.Do(req)
	// If you get an error here, make sure something else isn't running on same port
	chkErr(t, err)
	<-done
}

func chkErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func formDummyNode(t *testing.T) node.Node {
	return &nodeutil.Basic{
		OnAction: func(r node.ActionRequest) (node.Node, error) {
			sel, err := r.Input.Find("a")
			chkErr(t, err)
			v, err := sel.Get()
			chkErr(t, err)
			if v.String() != "hello" {
				t.Error(v.String())
			}

			sel, err = r.Input.Find("b")
			chkErr(t, err)
			v, err = sel.Get()
			chkErr(t, err)
			rdr, valid := v.Value().(io.Reader)
			if !valid {
				panic("invalid")
			}
			actual, err := ioutil.ReadAll(rdr)
			chkErr(t, err)
			if string(actual) != "hello world" {
				t.Error(actual)
			}
			//defer rdr.Close()
			fmt.Print(string(actual))
			return nil, nil
		},
	}
}
