package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"
)

//go:embed templates
var templateFS embed.FS

var tmpl *template.Template

func init() {
	tmpl = template.Must(template.ParseFS(templateFS, "**/*.gohtml"))
}

type InMemorySession struct {
	session map[string]string
}

var DefaultSessionBackend = InMemorySession{make(map[string]string)}

type FormErrors map[string]string

type ContactForm struct {
	Name    string     `form:"name"`
	Email   string     `form:"email"`
	Message string     `form:"message"`
	Errors  FormErrors `form:"-"`
}

func (cf *ContactForm) validate() FormErrors {
	if cf.Name == "" {
		cf.Errors["name"] = "name field is required"
	}
	if cf.Email == "" {
		cf.Errors["email"] = "email field is required"
	}
	if cf.Message == "" {
		cf.Errors["message"] = "message field is required"
	}
	return cf.Errors
}

func parseContactForm(r *http.Request) (*ContactForm, error) {
	name := r.FormValue("name")
	email := r.FormValue("email")
	message := r.FormValue("message")

	cf := &ContactForm{name, email, message, FormErrors{}}

	if nerrs := len(cf.validate()); nerrs != 0 {
		return cf, errors.New("error while parsing form")
	}
	return cf, nil
}

func mustRender(w io.Writer, templ string, data interface{}) {
	if err := tmpl.ExecuteTemplate(w, templ, data); err != nil {
		panic(err)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	formErrors, _ := r.Context().Value(Key("form-errors")).(FormErrors)
	messages, _ := r.Context().Value(Key("flash")).(string)
	fm := DecodeFlashMessage(messages)
	data := make(map[string]interface{})

	if len(formErrors) > 0 {
		data["formErrors"] = formErrors
	}
	if len(fm) > 0 {
		data["messages"] = fm
	}
	mustRender(w, "index.gohtml", data)
}

func handleContactForm(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		cf, err := parseContactForm(r)

		fm := FlashMessage{}
		if err != nil {
			fm["error"] = "An error occured while processing your request."
			ctx := context.WithValue(r.Context(), Key("form-errors"), cf.Errors)
			r = r.WithContext(ctx)
		} else {
			msg := fmt.Sprintf("Thanks for reaching out %s, your message has been recorded successfully and I typically respond within 48 working hours.", cf.Name)
			fm["success"] = msg
		}
		addFlashMessage(w, fm)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

type Key string

func MessageMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("flash")
		if err == nil {
			fm := DefaultSessionBackend.session[cookie.Value]
			ctx := context.WithValue(r.Context(), Key("flash"), fm)
			r = r.WithContext(ctx)
			cookie.MaxAge = -1
		}
		h.ServeHTTP(w, r)
	})
}

type FlashMessage map[string]string

func (fm FlashMessage) String() string {
	buf := &bytes.Buffer{}
	gob.NewEncoder(buf).Encode(fm)
	str := buf.String()
	wc := base64.NewEncoder(base64.URLEncoding, buf)
	buf.Reset()
	wc.Write([]byte(str))
	wc.Close()
	return buf.String()
}

func DecodeFlashMessage(s string) FlashMessage {
	sr := strings.NewReader(s)
	rr := base64.NewDecoder(base64.URLEncoding, sr)
	data, _ := io.ReadAll(rr)
	sr.Reset(string(data))
	fm := FlashMessage{}
	_ = gob.NewDecoder(sr).Decode(&fm)
	return fm
}

func addFlashMessage(w http.ResponseWriter, fm FlashMessage) {
	val := fm.String()
	key := val[:12]

	c := http.Cookie{
		Name:   "flash",
		Value:  key,
		MaxAge: 10,
	}

	DefaultSessionBackend.session[key] = val
	http.SetCookie(w, &c)
}
