package main

import (
	. "./model"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
)

var createTableStatements = []string{
	`CREATE DATABASE IF NOT EXISTS library DEFAULT CHARACTER SET = 'utf8' DEFAULT COLLATE 'utf8_general_ci';`,
	`USE library;`,
	`CREATE TABLE IF NOT EXISTS books (
		id INT UNSIGNED NOT NULL AUTO_INCREMENT,
		title VARCHAR(255) NULL,
		author VARCHAR(255) NULL,
		publishedDate VARCHAR(255) NULL,
		imageUrl VARCHAR(255) NULL,
		description TEXT NULL,
		createdBy VARCHAR(255) NULL,
		createdById VARCHAR(255) NULL,
		PRIMARY KEY (id)
	)`,
}

const (
	listStatement   = "SELECT * FROM books ORDER BY title"
	listByStatement = "SELECT * FROM books WHERE createdById = ? ORDER BY title"
	getStatement    = "SELECT * FROM books WHERE id = ?"
	insertStatement = "INSERT INTO books (title, author, publishedDate, imageUrl, description, createdBy, createdById) " +
		"VALUES (?, ?, ?, ?, ?, ?, ?)"
	deleteStatement = "DELETE FROM books WHERE id = ?"
	updateStatement = "UPDATE books SET title = ?, author = ?, publishedDate = ?, imageUrl = ?, description = ?, " +
		"createdBy = ?, createdById = ? WHERE id = ?"
)

type mysqlDB struct {
	conn *sql.DB

	list   *sql.Stmt
	listBy *sql.Stmt
	insert *sql.Stmt
	get    *sql.Stmt
	update *sql.Stmt
	delete *sql.Stmt
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

var _ BookDatabase = &mysqlDB{}

type MySQLConfig struct {
	Username, Password string

	Host string

	Port int

	UnixSocket string
}

func (c *MySQLConfig) dataStoreName(databaseName string) string {

	var cred string

	if c.Username != "" {
		cred = c.Username
		if c.Password != "" {
			cred = cred + ":" + c.Password
		}
		cred = cred + "@"
	}

	if c.UnixSocket != "" {
		return fmt.Sprintf("%sunix(%s)/%s", cred, c.UnixSocket, databaseName)
	}

	return fmt.Sprintf("%stcp([%s]:%d)/%s", cred, c.Host, c.Port, databaseName)
}

func (c *MySQLConfig) ensureTableExists() error {
	conn, err := sql.Open("mysql", c.dataStoreName(""))

	if err != nil {
		return fmt.Errorf("mysql: could not get a connection: %v", err)
	}

	defer conn.Close()

	if conn.Ping() == driver.ErrBadConn {
		return fmt.Errorf("mysql: could not connect to the database. " +
			"could be bad address, or this address is not whitelisted for access.")
	}

	if _, err := conn.Exec("USE library"); err != nil {
		if mErr, ok := err.(*mysql.MySQLError); ok && mErr.Number == 1049 {
			return createTable(conn)
		}
	}

	if _, err := conn.Exec("DESCRIBE books"); err != nil {
		if mErr, ok := err.(*mysql.MySQLError); ok && mErr.Number == 1146 {
			return createTable(conn)
		}

		return fmt.Errorf("mysql: could not connect to the database:%v", err)
	}

	return nil
}

func createTable(conn *sql.DB) error {
	for _, stmt := range createTableStatements {
		_, err := conn.Exec(stmt)

		if err != nil {
			return err
		}
	}

	return nil
}

func newMySQLDB(config MySQLConfig) (BookDatabase, error) {

	if err := config.ensureTableExists(); err != nil {
		return nil, err
	}

	conn, err := sql.Open("mysql", config.dataStoreName("library"))

	if err != nil {
		return nil, fmt.Errorf("mysql: could not get a connetion: %v", err)
	}

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("mysql; could not establish a good connection:%v", err)
	}

	db := &mysqlDB{
		conn: conn,
	}

	if db.list, err = conn.Prepare(listStatement); err != nil {
		return nil, fmt.Errorf("mysql: prepare list: %v", err)
	}

	if db.listBy, err = conn.Prepare(listByStatement); err != nil {
		return nil, fmt.Errorf("mysql: prepare listBy: %v", err)
	}

	if db.get, err = conn.Prepare(getStatement); err != nil {
		return nil, fmt.Errorf("mysql: prepare get: %v", err)
	}

	if db.insert, err = conn.Prepare(insertStatement); err != nil {
		return nil, fmt.Errorf("mysql: prepare insert: %v", err)
	}

	if db.update, err = conn.Prepare(updateStatement); err != nil {
		return nil, fmt.Errorf("mysql: prepare update: %v", err)
	}

	if db.delete, err = conn.Prepare(deleteStatement); err != nil {
		return nil, fmt.Errorf("mysql: prepare delete: %v", err)
	}

	return db, nil

}

func (db *mysqlDB) Close() {
	db.conn.Close()
}

func scanBook(s rowScanner) (*Book, error) {
	var (
		id            int64
		title         sql.NullString
		author        sql.NullString
		publishedDate sql.NullString
		imageURL      sql.NullString
		description   sql.NullString
		createdBy     sql.NullString
		createdByID   sql.NullString
	)

	if err := s.Scan(&id, &title, &author, &publishedDate, &imageURL,
		&description, &createdBy, &createdByID); err != nil {
		return nil, err
	}

	book := &Book{
		ID:            id,
		Title:         title.String,
		Author:        author.String,
		PublishedDate: publishedDate.String,
		ImageURL:      imageURL.String,
		Description:   description.String,
		CreatedBy:     createdBy.String,
		CreatedByID:   createdByID.String,
	}

	return book, nil

}

func execAffectingOneRow(stmt *sql.Stmt, args ...interface{}) (sql.Result, error) {
	r, err := stmt.Exec(args...)

	if err != nil {
		return r, fmt.Errorf("mysql: could not execute statement: %v", err)
	}

	rowsAffected, err := r.RowsAffected()

	if err != nil {
		return r, fmt.Errorf("mysql: could not get rows affected: %v", err)
	} else if rowsAffected != 1 {
		return r, fmt.Errorf("mysql: expected 1 row affected, got %d", rowsAffected)
	}

	return r, nil
}

func (db *mysqlDB) ListBooks() ([]*Book, error) {
	rows, err := db.list.Query()
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var books []*Book
	for rows.Next() {
		book, err := scanBook(rows)
		if err != nil {
			return nil, fmt.Errorf("msql: could not read row: %v", err)
		}

		books = append(books, book)
	}

	return books, nil

}

func (db *mysqlDB) ListBooksCreatedBy(userID string) ([]*Book, error) {
	if userID == "" {
		return db.ListBooks()
	}

	rows, err := db.listBy.Query(userID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var books []*Book

	for rows.Next() {
		book, err := scanBook(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: could not read row: %v", err)
		}

		books = append(books, book)
	}

	return books, nil

}

func (db *mysqlDB) GetBook(id int64) (*Book, error) {
	book, err := scanBook(db.get.QueryRow(id))

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("mysql: could not find book with id %d", id)
	}

	if err != nil {
		return nil, fmt.Errorf("mysql: could not get book: %v", err)
	}

	return book, nil
}

func (db *mysqlDB) AddBook(b *Book) (id int64, err error) {
	r, err := execAffectingOneRow(db.insert, b.Title, b.Author, b.PublishedDate, b.ImageURL, b.Description, b.CreatedBy, b.CreatedByID)

	if err != nil {
		return 0, err
	}

	lastInsertID, err := r.LastInsertId()

	if err != nil {
		return 0, fmt.Errorf("mysql: could not get last insert ID: %V", err)
	}

	return lastInsertID, nil
}

func (db *mysqlDB) DeleteBook(id int64) error {
	if id == 0 {
		return errors.New("mysql: book with unassigned ID passed into deleteBook")
	}

	_, err := execAffectingOneRow(db.delete, id)

	return err
}

func (db *mysqlDB) UpdateBook(b *Book) error {
	if b.ID == 0 {
		return errors.New("mysql: book with unassigned ID passed into updateBook")
	}

	_, err := execAffectingOneRow(db.update, b.Title, b.Author, b.PublishedDate, b.ImageURL, b.Description, b.CreatedBy, b.CreatedByID, b.ID)
	return err
}

func main() {
	config := MySQLConfig{Username: "root", Password: "12345678", Port: 3306, Host: "127.0.0.1"}
	database, err := newMySQLDB(config)

	if err != nil {
		fmt.Println("发生错误")
	}
	book1 := &Book{
		ID:            110,
		Title:         "小朋友",
		Author:        "小朋友",
		PublishedDate: "2018-06-28",
		ImageURL:      "http://www.baidu.com",
		Description:   "哈哈啊哈哈哈哈啊",
		CreatedBy:     "tiny",
		CreatedByID:   "110",
	}
	database.AddBook(book1)
	book, err := database.GetBook(2)

	fmt.Println(book.ID, book.Title, book.Author, book.Description)

}
