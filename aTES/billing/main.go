package main

import (
	"billing/db"
	"billing/kafka/consumer"
	"billing/kafka/producer"
	"billing/webserver"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"text/template"
	"time"

	"github.com/astaxie/session"
	_ "github.com/astaxie/session/providers/memory"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

var (
	port   = 3003
	config = oauth2.Config{
		ClientID:     "b8ULvQgqpBI5MHl4TBSvaes2OMZDcPMvQYVur-_Hw_M",
		ClientSecret: "PwSu2B3A2I1FseVJvXrM2VcZS8pmNOKl8Gd6RlH7H18",
		RedirectURL:  "http://localhost:3003/oauth2",
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
	t = template.Must(template.ParseFiles("templates/home.html"))
}

type Server struct {
	*webserver.Server
	dbConn   db.Connection
	producer *producer.Producer
	bcId     uuid.UUID
}

func main() {
	srv := Server{
		Server: webserver.New(port),
		dbConn: db.Connect(),
	}

	bcs, err := srv.dbConn.GetAllBillingCycles()
	if err != nil {
		panic(err)
	}
	if len(bcs) > 1 {
		panic("should not happen")
	} else if len(bcs) == 0 {
		srv.bcId = uuid.New()
		if err := srv.dbConn.CreateBillingCycle(&db.BillingCycle{
			PublicID: srv.bcId,
			DaySum:   0,
		}); err != nil {
			panic(err)
		}
	}

	srv.AddHandler("/login", login)
	srv.AddHandler("/oauth2", oauth)

	srv.AddHandle("/", authHandler(http.HandlerFunc(srv.home)))
	srv.AddHandle("/analytics", authHandler(http.HandlerFunc(srv.analytics)))

	srv.producer = producer.NewProducer()
	go func() {
		srv.producer.Run()
	}()

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

		consumer.NewConsumer(srv.dbConn, ch, *srv.producer).Run()
	}()

	go func() {
		for {
			srv.closeBillingCycle()
			time.Sleep(time.Hour * 24)
		}
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

func getUserInfo(accessToken string) *db.BillingAccount {
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

	var acc db.BillingAccount
	if err := json.Unmarshal(data, &acc); err != nil {
		log.Println("Unmarshal acc", err)
	}

	return &acc
}

func getCurrentUser(w http.ResponseWriter, r *http.Request) (*db.BillingAccount, error) {
	sess := globalSessions.SessionStart(w, r)
	acc := sess.Get("account")
	if acc == nil {
		return nil, errors.New("no account")
	}
	if a, ok := acc.(db.BillingAccount); ok {
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

func internalError(w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("Internal Error"))
}

func (srv *Server) analytics(w http.ResponseWriter, r *http.Request) {
	user, err := getCurrentUser(w, r)
	if err != nil {
		internalError(w)
		return
	}

	if user.Role == nil || *user.Role != db.Role_Admin {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Forbidden"))
		return
	}

	bc, err := srv.dbConn.GetBillingCycle(srv.bcId.String())
	if err != nil {
		log.Println("failed to get all txses", err)
		internalError(w)
		return
	}

	stats := make(map[string]int)
	allTasks, err := srv.dbConn.GetDoneBillingTasks()
	if err != nil {
		log.Println("failed to get all btasks", err)
		internalError(w)
		return
	}
	for _, task := range allTasks {
		day := task.CloseDay.String()
		if stats[day] < task.DoneCost {
			stats[day] = task.DoneCost
		}
	}

	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	e.Encode(struct {
		DaySum    int            `json:"day_sum"`
		TaskStats map[string]int `json:"tasks_stats"`
	}{
		DaySum:    bc.DaySum,
		TaskStats: stats,
	})
}

func (srv *Server) home(w http.ResponseWriter, r *http.Request) {
	user, err := getCurrentUser(w, r)
	if err != nil {
		internalError(w)
		return
	}

	if user.Role == nil || (*user.Role != db.Role_Admin && *user.Role != db.Role_Accounter) {
		txs, err := srv.dbConn.GetAllUserTransactions(user.TransactionLog)
		if err != nil {
			log.Println("failed to get user txses", err)
			internalError(w)
			return
		}

		t.ExecuteTemplate(w, "home", struct {
			Transactions []db.Transaction
			Balance      int
		}{
			Transactions: txs,
			Balance:      user.Balance,
		})
		return
	}

	bc, err := srv.dbConn.GetBillingCycle(srv.bcId.String())
	if err != nil {
		log.Println("failed to get all txses", err)
		internalError(w)
		return
	}
	txs, err := srv.dbConn.GetAllUserTransactions(bc.TransactionLog)
	if err != nil {
		log.Println("failed to get user txses", err)
		internalError(w)
		return
	}
	t.ExecuteTemplate(w, "home", struct {
		Transactions []db.Transaction
		Balance      int
	}{
		Transactions: txs,
		Balance:      bc.DaySum,
	})
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

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func login(w http.ResponseWriter, r *http.Request) {
	u := config.AuthCodeURL("")
	http.Redirect(w, r, u, http.StatusFound)
}

func (srv *Server) closeBillingCycle() error {
	tx := &db.Transaction{
		PublicID: uuid.New(),
		OwnerID:  srv.bcId,
		Cost:     0,
		Type:     db.TxType_MakePayment,
	}
	if err := srv.dbConn.CreateTransaction(tx); err != nil {
		return err
	}
	if err := srv.producer.TxCreatedMsg(*tx); err != nil {
		log.Println("failed to publish!", err)
	}

	users, err := srv.dbConn.GetAllAccounts()
	if err != nil {
		return err
	}

	for _, user := range users {
		user.TransactionLog = append(user.TransactionLog, tx.PublicID.String())
		// TODO payout
		if err := srv.dbConn.SaveAccount(&user); err != nil {
			return err
		}
	}

	bc, err := srv.dbConn.GetBillingCycle(srv.bcId.String())
	if err != nil {
		return err
	}
	bc.TransactionLog = append(bc.TransactionLog, tx.PublicID.String())
	bc.DaySum = 0
	return srv.dbConn.SaveBillingCycle(bc)
}
