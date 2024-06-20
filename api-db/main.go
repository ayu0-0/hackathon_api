package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oklog/ulid/v2"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

type UserResForHTTPGet struct {
	Id     string `json:"id"`
	Email  string `json:"email"`
	Userid string `json:"userid"`
	Name   string `json:"name"`
}

type Posts struct {
	Id        string    `json:"id"`
	UserId    string    `json:"user_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type Likes struct {
	Id        string    `json:"id"`
	UserId    string    `json:"user_id"`
	PostId    string    `json:"post_id"`
	CreatedAt time.Time `json:"created_at"`
}

type Replies struct {
	Id        string    `json:"id"`
	UserId    string    `json:"user_id"`
	PostId    string    `json:"post_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// ① GoプログラムからMySQLへ接続
var db *sql.DB

func init() {
	// ①-1
	err := godotenv.Load(".env")

	// もし err がnilではないなら、"読み込み出来ませんでした"が出力されます。
	if err != nil {
		fmt.Printf("読み込み出来ませんでした: %v", err)
	}

	// DB接続のための準備
	mysqlUser := os.Getenv("MYSQL_USER")
	mysqlPwd := os.Getenv("MYSQL_PWD")
	mysqlHost := os.Getenv("MYSQL_HOST")
	mysqlDatabase := os.Getenv("MYSQL_DATABASE")

	connStr := fmt.Sprintf("%s:%s@%s/%s", mysqlUser, mysqlPwd, mysqlHost, mysqlDatabase)
	_db, err := sql.Open("mysql", connStr)
	if err != nil {
		log.Fatalf("fail: sql.Open, %v\n", err)
	}
	// ①-3
	if err := _db.Ping(); err != nil {
		log.Fatalf("fail: _db.Ping, %v\n", err)
	}
	db = _db
}

func repliesHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With, Authorization, body") // リクエストヘッダーの許可設定
		w.WriteHeader(http.StatusOK)
		return
	}

	switch r.Method {
	case http.MethodOptions:
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With, Authorization, body")
		w.WriteHeader(http.StatusOK)

	case http.MethodGet:
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		// ②-2
		rows, err := db.Query("SELECT id, post_id, user_id, content, created_at FROM replies")
		if err != nil {
			log.Printf("fail: db.Query, %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// ②-3
		replies := make([]Replies, 0)
		for rows.Next() {
			var r Replies
			var createdAt []uint8
			if err := rows.Scan(&r.Id, &r.PostId, &r.UserId, &r.Content, &createdAt); err != nil {
				log.Printf("fail: rows.Scan, %v\n", err)

				if err := rows.Close(); err != nil { // 500を返して終了するが、その前にrowsのClose処理が必要
					log.Printf("fail: rows.Close(), %v\n", err)
				}
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			// createdAt を time.Time 型に変換
			r.CreatedAt, err = time.Parse("2006-01-02 15:04:05", string(createdAt))
			if err != nil {
				log.Printf("fail: time.Parse, %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			replies = append(replies, r)
		}

		// ②-4
		bytes, err := json.Marshal(replies)
		if err != nil {
			log.Printf("fail: json.Marshal, %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(bytes)

	case http.MethodPost:
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With, Authorization, body")

		// リクエストボディをJSON形式でデコード
		var newReply Replies
		err := json.NewDecoder(r.Body).Decode(&newReply)
		if err != nil {
			log.Printf("fail: json decode, %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if len(newReply.Content) > 150 || newReply.Content == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		t := time.Now()
		entropy := ulid.Monotonic(rand.New(rand.NewSource(t.UnixNano())), 0)
		ulid := ulid.MustNew(ulid.Timestamp(t), entropy)

		// ULIDを文字列として表示
		fmt.Println(ulid.String())

		response := struct {
			ID string `json:"id"`
		}{
			ID: ulid.String(),
		}

		json.NewEncoder(w).Encode(response)

		// データベースに新しいユーザーを挿入
		log.Printf("Inserting post with id: %s, post_id: %s, user_id: %s, content: %s, created_at: %v", ulid.String(), newReply.PostId, newReply.UserId, newReply.Content, t)

		_, err = db.Exec("INSERT INTO replies (id,post_id, user_id,content, created_at) VALUES (?, ?, ?, ?, ?)", ulid.String(), newReply.PostId, newReply.UserId, newReply.Content, t)
		if err != nil {
			log.Printf("fail: db.Exec, %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated) // リソースの作成を通知

		fmt.Fprintf(w, "Reply %s created successfully", newReply.Content)

	default:
		log.Printf("fail: HTTP Method is %s\n", r.Method)
		w.WriteHeader(http.StatusBadRequest)
		return

	}
}

func likesHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With, Authorization, body") // リクエストヘッダーの許可設定
		w.WriteHeader(http.StatusOK)
		return
	}

	switch r.Method {
	case http.MethodOptions:
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With, Authorization, body")
		w.WriteHeader(http.StatusOK)

	case http.MethodGet:
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		// ②-2
		rows, err := db.Query("SELECT id, user_id, post_id, created_at FROM likes")
		if err != nil {
			log.Printf("fail: db.Query, %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// ②-3
		likes := make([]Likes, 0)
		for rows.Next() {
			var l Likes
			var createdAt string
			if err := rows.Scan(&l.Id, &l.UserId, &l.PostId, &createdAt); err != nil {
				log.Printf("fail: rows.Scan, %v\n", err)

				if err := rows.Close(); err != nil { // 500を返して終了するが、その前にrowsのClose処理が必要
					log.Printf("fail: rows.Close(), %v\n", err)
				}
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			// createdAt を time.Time 型に変換
			l.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
			if err != nil {
				log.Printf("fail: time.Parse, %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			likes = append(likes, l)
		}

		// ②-4
		bytes, err := json.Marshal(likes)
		if err != nil {
			log.Printf("fail: json.Marshal, %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(bytes)

	case http.MethodPost:
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With, Authorization, body")

		// リクエストボディをJSON形式でデコード
		var newLike Likes
		err := json.NewDecoder(r.Body).Decode(&newLike)
		if err != nil {
			log.Printf("fail: json decode, %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		t := time.Now()
		entropy := ulid.Monotonic(rand.New(rand.NewSource(t.UnixNano())), 0)
		ulid := ulid.MustNew(ulid.Timestamp(t), entropy)

		// ULIDを文字列として表示
		fmt.Println(ulid.String())

		response := struct {
			ID string `json:"id"`
		}{
			ID: ulid.String(),
		}

		json.NewEncoder(w).Encode(response)

		// データベースに新しいユーザーを挿入
		log.Printf("Inserting likes with id: %s, user_id: %s, post_id: %s, created_at: %v", ulid.String(), newLike.UserId, newLike.PostId, t)

		_, err = db.Exec("INSERT INTO likes (id,user_id,post_id, created_at) VALUES (?, ?, ?, ?)", ulid.String(), newLike.UserId, newLike.PostId, t)
		if err != nil {
			log.Printf("fail: db.Exec, %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated) // リソースの作成を通知

		fmt.Fprintf(w, "Post %s created successfully", newLike.Id)

	default:
		log.Printf("fail: HTTP Method is %s\n", r.Method)
		w.WriteHeader(http.StatusBadRequest)
		return

	}
}

func postsHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With, Authorization, body") // リクエストヘッダーの許可設定
		w.WriteHeader(http.StatusOK)
		return
	}

	switch r.Method {
	case http.MethodOptions:
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With, Authorization, body")
		w.WriteHeader(http.StatusOK)

	case http.MethodGet:
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		// ②-1
		//name := r.URL.Query().Get("name")

		/*
			if name == "" {
				log.Println("fail: name is empty")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		*/

		// ②-2
		rows, err := db.Query("SELECT id, user_id, content, created_at FROM posts")
		if err != nil {
			log.Printf("fail: db.Query, %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// ②-3
		posts := make([]Posts, 0)
		for rows.Next() {
			var p Posts
			var createdAt []uint8
			if err := rows.Scan(&p.Id, &p.UserId, &p.Content, &createdAt); err != nil {
				log.Printf("fail: rows.Scan, %v\n", err)

				if err := rows.Close(); err != nil { // 500を返して終了するが、その前にrowsのClose処理が必要
					log.Printf("fail: rows.Close(), %v\n", err)
				}
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			// createdAt を time.Time 型に変換
			p.CreatedAt, err = time.Parse("2006-01-02 15:04:05", string(createdAt))
			if err != nil {
				log.Printf("fail: time.Parse, %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			posts = append(posts, p)
		}

		// ②-4
		bytes, err := json.Marshal(posts)
		if err != nil {
			log.Printf("fail: json.Marshal, %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(bytes)

	case http.MethodPost:
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With, Authorization, body")

		// リクエストボディをJSON形式でデコード
		var newPost Posts
		err := json.NewDecoder(r.Body).Decode(&newPost)
		if err != nil {
			log.Printf("fail: json decode, %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if len(newPost.Content) > 150 || newPost.Content == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		t := time.Now()
		entropy := ulid.Monotonic(rand.New(rand.NewSource(t.UnixNano())), 0)
		ulid := ulid.MustNew(ulid.Timestamp(t), entropy)

		// ULIDを文字列として表示
		fmt.Println(ulid.String())

		response := struct {
			ID string `json:"id"`
		}{
			ID: ulid.String(),
		}

		json.NewEncoder(w).Encode(response)

		// データベースに新しいユーザーを挿入
		log.Printf("Inserting post with id: %s, user_id: %s, content: %s, created_at: %v", ulid.String(), newPost.UserId, newPost.Content, t)

		_, err = db.Exec("INSERT INTO posts (id,user_id,content, created_at) VALUES (?, ?, ?, ?)", ulid.String(), newPost.UserId, newPost.Content, t)
		if err != nil {
			log.Printf("fail: db.Exec, %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated) // リソースの作成を通知

		fmt.Fprintf(w, "Post %s created successfully", newPost.Content)

	default:
		log.Printf("fail: HTTP Method is %s\n", r.Method)
		w.WriteHeader(http.StatusBadRequest)
		return

	}
}

// ② /userでリクエストされたらnameパラメーターと一致する名前を持つレコードをJSON形式で返す
func handler(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With, Authorization, body") // リクエストヘッダーの許可設定
		w.WriteHeader(http.StatusOK)
		return
	}

	switch r.Method {
	case http.MethodOptions:
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With, Authorization, body")
		w.WriteHeader(http.StatusOK)

	case http.MethodGet:
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		// ②-1
		//name := r.URL.Query().Get("name")

		/*
			if name == "" {
				log.Println("fail: name is empty")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		*/

		// ②-2
		rows, err := db.Query("SELECT id, name, userid FROM users")
		if err != nil {
			log.Printf("fail: db.Query, %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// ②-3
		users := make([]UserResForHTTPGet, 0)
		for rows.Next() {
			var u UserResForHTTPGet
			if err := rows.Scan(&u.Id, &u.Name, &u.Userid); err != nil {
				log.Printf("fail: rows.Scan, %v\n", err)

				if err := rows.Close(); err != nil { // 500を返して終了するが、その前にrowsのClose処理が必要
					log.Printf("fail: rows.Close(), %v\n", err)
				}
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			users = append(users, u)
		}

		// ②-4
		bytes, err := json.Marshal(users)
		if err != nil {
			log.Printf("fail: json.Marshal, %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(bytes)

	case http.MethodPost:
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With, Authorization, body")

		// リクエストボディをJSON形式でデコード
		var newUser UserResForHTTPGet
		err := json.NewDecoder(r.Body).Decode(&newUser)
		if err != nil {
			log.Printf("fail: json decode, %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if len(newUser.Name) > 50 || newUser.Name == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if newUser.Email == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		//
		//t := time.Now()
		//entropy := ulid.Monotonic(rand.New(rand.NewSource(t.UnixNano())), 0)
		//ulid := ulid.MustNew(ulid.Timestamp(t), entropy)
		//
		//// ULIDを文字列として表示
		//fmt.Println(ulid.String())
		//
		//response := struct {
		//	ID string `json:"id"`
		//}{
		//	ID: ulid.String(),
		//}
		//
		//json.NewEncoder(w).Encode(response)

		log.Printf(newUser.Id)

		// データベースに新しいユーザーを挿入
		_, err = db.Exec("INSERT INTO users (id,email, userid,name) VALUES (?, ?, ?, ?)", newUser.Id, newUser.Email, newUser.Userid, newUser.Name)
		if err != nil {
			log.Printf("fail: db.Exec, %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated) // リソースの作成を通知

		fmt.Fprintf(w, "User %s created successfully", newUser.Name)

	default:
		log.Printf("fail: HTTP Method is %s\n", r.Method)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
}

func main() {
	// ② /userでリクエストされたらnameパラメーターと一致する名前を持つレコードをJSON形式で返す
	http.HandleFunc("/users", handler)
	http.HandleFunc("/posts", postsHandler)
	http.HandleFunc("/likes", likesHandler)
	http.HandleFunc("/replies", repliesHandler)

	// ③ Ctrl+CでHTTPサーバー停止時にDBをクローズする
	closeDBWithSysCall()

	// 8000番ポートでリクエストを待ち受ける
	log.Println("Listening...")
	if err := http.ListenAndServe(":8000", nil); err != nil {
		log.Fatal(err)
	}
}

// ③ Ctrl+CでHTTPサーバー停止時にDBをクローズする
func closeDBWithSysCall() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		s := <-sig
		log.Printf("received syscall, %v", s)

		if err := db.Close(); err != nil {
			log.Fatal(err)
		}
		log.Printf("success: db.Close()")
		os.Exit(0)
	}()
}
