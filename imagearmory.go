package main

import (
	"encoding/base64"
	"fmt"
	"github.com/DeftNerd/imagearmory/server"
	"github.com/codegangsta/cli"
	"github.com/codegangsta/negroni"
	"github.com/nu7hatch/gouuid"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

const BUCKETNAME = "justatestingbucket2"
const DATAPATH = "data/"
const HTTPSTOREPATH = "/store"
const HTTPGETPATH = "/get/"
const RESOURCEPATH = "/c/"
const MAINPATH = "/"
const UIFILE = "client/index.html"
const UIPATH = "client/"
const UMASK = 0664

type webFunc func(http.ResponseWriter, *http.Request)

func GetId() string {
	u4, err := uuid.NewV4()
	if err != nil {
		log.Fatalf("Unable to generate UUID: %v\n", err)
	}

	var byteData []byte = make([]byte, 16)
	copy(byteData, u4[:])

	// Base64 encode, remove ==s and replace slashes
	res := strings.Trim(base64.StdEncoding.EncodeToString(byteData), "=")

	return strings.Replace(res, "/", "x", -1)
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		return false
	}
	return true
}

func StoreHandler(store server.ObjectStore) webFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := GetId()

		err := store.Put(id, []byte(r.FormValue("r")))
		if err != nil {
			log.Printf("%v\n", err)
		}

		fmt.Fprintf(w, "OK:%s", id)
	}
}

func GetHandler(store server.ObjectStore) webFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fname := r.URL.Path[len(HTTPGETPATH):]

		if store.IsLocal() {
			buffer, err := store.Get(fname)
			if err == nil {
				fmt.Fprint(w, string(buffer))
				return
			}
		} else {
			http.Redirect(w, r, store.GetURL(fname), http.StatusFound)
		}
		fmt.Fprint(w, "FAIL")
	}
}

func Mainhandler(w http.ResponseWriter, r *http.Request) {
	ui, err := ioutil.ReadFile(UIFILE)
	if err != nil {
		fmt.Fprint(w, "Couldn't read user interface")
		return
	}
	fmt.Fprint(w, string(ui))
}

func Resourcehandler(w http.ResponseWriter, r *http.Request) {
	fname := r.URL.Path[len(RESOURCEPATH):]
	if fname == "" {
		http.Error(w, "404 File not found", http.StatusNotFound)
		return
	}
	if strings.Contains(fname, "..") || fname[:1] == "/" {
		http.Error(w, "Nice try.", http.StatusForbidden)
		return
	}
	fname = UIPATH + fname
	if FileExists(fname) {
		ui, err := ioutil.ReadFile(fname)
		if err == nil {
			fmt.Fprint(w, string(ui))
			return
		}
	}
	http.Error(w, "404 File not found", http.StatusNotFound)
}

func initializeStorage(c *cli.Context) (store server.ObjectStore) {
	switch c.String("storage") {
	case "local":
		log.Fatal("Local storage not yet re-implemented!")
	case "s3":
		store = &server.S3Adapter{}
	default:
		log.Fatalf("Undefined storage class '%v'\n", c.String("storage"))
	}

	store.Init(c)

	return
}

func main() {
	app := cli.NewApp()

	app.Name = "imagearmory"
	app.Version = "0.1"
	app.Usage = "Encrypted image server"
	app.Flags = []cli.Flag{
		cli.StringFlag{"port, p", "8080", "Server port"},
		cli.StringFlag{"storage", "local", "Data storage backend (local, s3)"},
		cli.StringFlag{"bucket", "", "Target S3 bucket"},
	}

	app.Action = func(c *cli.Context) {
		store := initializeStorage(c)

		mux := http.NewServeMux()

		mux.HandleFunc(HTTPSTOREPATH, StoreHandler(store))
		mux.HandleFunc(HTTPGETPATH, GetHandler(store))
		mux.HandleFunc(RESOURCEPATH, Resourcehandler)
		mux.HandleFunc(MAINPATH, Mainhandler)

		static := negroni.NewStatic(http.Dir("client"))
		static.Prefix = "/c"

		n := negroni.Classic()
		n.Use(static)
		n.UseHandler(mux)
		n.Run(":8080")
	}

	app.Run(os.Args)
}
