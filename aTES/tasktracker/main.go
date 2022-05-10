package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"tasktracker/db"
	"tasktracker/kafka/consumer"
	"tasktracker/kafka/producer"
	"tasktracker/webserver"
	"text/template"

	"github.com/astaxie/session"
	_ "github.com/astaxie/session/providers/memory"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

var (
	port   = 3002
	config = oauth2.Config{
		ClientID:     "Rk_m1XFqk_dUlugR60f3KwrwkPzBtr6wY9gH5vuBqik",
		ClientSecret: "uyEOplK0cqlt0k58Wh7GI8w986oSvhz73mKJC9mNrhU",
		RedirectURL:  "http://localhost:3002/oauth2",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "http://localhost:3000/oauth/authorize",
			TokenURL: "http://oauth:3000/oauth/token",
		},
		Scopes: []string{"all"},
	}
	globalSessions   *session.Manager
	accountsToUpdate map[uuid.UUID]bool
	mux              sync.Mutex
	t                *template.Template
)

func init() {
	accountsToUpdate = make(map[uuid.UUID]bool)
	globalSessions, _ = session.NewManager("memory", "gosessionid", 3600)
	go globalSessions.GC()
	t = template.Must(template.ParseFiles("templates/task_list.html", "templates/task_create.html"))
}

type Server struct {
	*webserver.Server
	dbConn   db.Connection
	producer *producer.Producer
}

func main() {
	srv := Server{
		Server: webserver.New(port),
		dbConn: db.Connect(),
	}
	srv.AddHandler("/login", login)
	srv.AddHandler("/oauth2", oauth)

	srv.AddHandle("/tasks", authHandler(http.HandlerFunc(srv.listTasks)))
	srv.AddHandle("/create", authHandler(http.HandlerFunc(srv.createTask)))
	srv.AddHandle("/update", authHandler(http.HandlerFunc(srv.updateTask)))
	srv.AddHandle("/shuffle", authHandler(http.HandlerFunc(srv.shuffleTasks)))

	go func() {
		ch := make(chan uuid.UUID, 10)
		go func() {
			for {
				uid := <-ch

				mux.Lock()
				accountsToUpdate[uid] = true
				mux.Unlock()
			}
		}()

		consumer.NewConsumer(srv.dbConn, ch).Run()
	}()

	srv.producer = producer.NewProducer()
	go func() {
		srv.producer.Run()
	}()

	log.Println("Running server....", "port", port)
	log.Println(srv.Run())
}

func authHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := globalSessions.SessionStart(w, r)
		ct := sess.Get("token")
		if ct == nil {
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func getUserInfo(accessToken string) *db.JiraAccount {
	req, err := http.NewRequest("GET", "http://oauth:3000/accounts/current", nil)
	if err != nil {
		log.Println(err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Add("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error on response.\n[ERRO] -", err)
		return nil
	}

	defer resp.Body.Close()
	data, _ := ioutil.ReadAll(resp.Body)

	var acc db.JiraAccount
	if err := json.Unmarshal(data, &acc); err != nil {
		log.Println("Unmarshal acc", err)
	}

	return &acc
}

func getCurrentUser(w http.ResponseWriter, r *http.Request) (*db.JiraAccount, error) {
	sess := globalSessions.SessionStart(w, r)
	acc := sess.Get("account")
	if acc == nil {
		return nil, errors.New("no account")
	}
	if a, ok := acc.(db.JiraAccount); ok {
		mux.Lock()
		upd := accountsToUpdate[a.PublicID]
		mux.Unlock()
		if upd {
			updatedAcc := getUserInfo(sess.Get("token").(string))
			if updatedAcc != nil {
				sess.Set("account", *updatedAcc)
				mux.Lock()
				accountsToUpdate[a.PublicID] = false
				mux.Unlock()
			}
		}
		return &a, nil
	}
	return nil, errors.New("bad val in acc session store")
}

func (srv *Server) shuffleTasks(w http.ResponseWriter, r *http.Request) {
	allTasks, err := srv.dbConn.GetAllTasks()
	if err != nil {
		log.Println("shuffle get all tasks err", err)
		internalError(w)
		return
	}
	allAccs, err := srv.dbConn.GetAllAccounts()
	if err != nil {
		log.Println("shuffle get all accs err", err)
		internalError(w)
		return
	}
	var accs []uuid.UUID
	for _, a := range allAccs {
		if a.Role == nil || *a.Role == db.Role_Worker {
			accs = append(accs, a.PublicID)
		}
	}

	if len(accs) == 0 || len(allTasks) == 0 {
		http.Redirect(w, r, "/tasks", http.StatusTemporaryRedirect)
		return
	}

	for _, t := range allTasks {
		if t.Status != db.Status_Done {
			t.OwnerID = accs[rand.Intn(len(accs))]
			if err := srv.dbConn.SaveTask(&t); err != nil {
				log.Println("task save failed", err)
			} else {
				srv.producer.TaskUpdatedMsg(t)
				srv.producer.TaskAssigned(t)
			}
		}
	}

	http.Redirect(w, r, "/tasks", http.StatusTemporaryRedirect)
}

func internalError(w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("Internal Error"))
}

func (srv *Server) updateTask(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	vals := r.Form
	if vals == nil || len(vals) < 1 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("No data was received. Please submit from the create page."))
		return
	}

	t, err := srv.dbConn.UpdateTask(vals.Get("public_id"), "", vals.Get("status"))
	if err != nil {
		log.Println("failed to update task", err)
		internalError(w)
		return
	} else {
		srv.producer.TaskUpdatedMsg(*t)
	}
	http.Redirect(w, r, "/tasks", http.StatusTemporaryRedirect)
}

func (srv *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	user, err := getCurrentUser(w, r)
	if err != nil {
		internalError(w)
		return
	}

	allTasks, err := srv.dbConn.GetAllTasks()
	if err != nil {
		log.Println("failed to get tasks", err)
		internalError(w)
		return
	}
	t.ExecuteTemplate(w, "list", struct {
		Tasks         []db.Task
		AllowReassign bool
	}{
		Tasks:         allTasks,
		AllowReassign: *user.Role == db.Role_Admin || *user.Role == db.Role_Manager,
	})
}

func (srv *Server) createTask(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		t.ExecuteTemplate(w, "create", nil)
		return
	}

	r.ParseForm()
	vals := r.Form
	if vals == nil || len(vals) < 1 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("No data was received. Please submit from the create page."))
		return
	}

	allAccs, err := srv.dbConn.GetAllAccounts()
	if err != nil {
		log.Println("shuffle get all accs err", err)
		internalError(w)
		return
	}
	var accs []uuid.UUID
	for _, a := range allAccs {
		if *a.Role == db.Role_Worker {
			accs = append(accs, a.PublicID)
		}
	}
	if len(accs) == 0 {
		w.Write([]byte("No workers to assign!"))
		return
	}

	t := &db.Task{PublicID: uuid.New(), Description: vals.Get("description"), OwnerID: accs[rand.Intn(len(accs))]}

	if err := srv.dbConn.Create(t); err != nil {
		log.Println(err)
	} else {
		srv.producer.TaskCreatedMsg(*t)
		srv.producer.TaskAssigned(*t)
	}
	log.Println("creating task with vals", vals)
	w.Write([]byte("Done!"))
}

func oauth(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	code := r.Form.Get("code")
	if code == "" {
		http.Error(w, "Code not found", http.StatusBadRequest)
		return
	}

	token, err := config.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ses := globalSessions.SessionStart(w, r)

	ses.Set("token", token.AccessToken)

	acc := getUserInfo(token.AccessToken)
	if acc != nil {
		ses.Set("account", *acc)
	}

	http.Redirect(w, r, "/tasks", http.StatusTemporaryRedirect)
}

func login(w http.ResponseWriter, r *http.Request) {
	u := config.AuthCodeURL("")
	http.Redirect(w, r, u, http.StatusFound)
}
