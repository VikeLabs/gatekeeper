package repository_test

import (
	"context"
	"gatekeeper/repository"
	"io/ioutil"
	"testing"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/jackc/pgx/v4"
)

var repo *repository.Repository
var conn *pgx.Conn

const EMAIL = "john@example.com"
const GUILDID = discord.GuildID(12345678901234567)
const USERID = discord.UserID(12345678901234567)

func TestCreateGuild(t *testing.T) {
	// guild doesn't exist
	var guildID discord.GuildID
	err := repo.CreateGuild(GUILDID)
	if err != nil {
		t.Error(err)
	}

	err = conn.QueryRow(context.Background(), "SELECT id FROM guild WHERE id = $1", GUILDID).Scan(&guildID)
	if err != nil {
		t.Error(err)
	}
	if guildID != GUILDID {
		t.Error("Guild ID doesn't match")
	}
	// guild already exists
	err = repo.CreateGuild(GUILDID)
	if err == nil {
		t.Error("Guild should already exist")
	}
	if err != repository.ErrGuildExists {
		t.Error("Error should be ErrGuildExists")
	}
}

func TestListGuildIDs(t *testing.T) {

	ids, err := repo.ListGuildIDs()
	if err != nil {
		t.Error(err)
	}
	if len(ids) == 0 {
		t.Error("No guilds found")
	}
	if ids[0] != GUILDID {
		t.Error("Guild ID doesn't match")
	}
}

func TestSetVerifiedEmail(t *testing.T) {
	err := repo.SetVerifiedEmail(GUILDID, USERID, EMAIL)
	if err != nil {
		t.Error(err)
	}
	conn.Exec(context.Background(), "DELETE FROM email WHERE guild_id = $1", GUILDID)
}

func TestGetVerifiedEmail(t *testing.T) {
	// existing email
	userID, err := repo.GetVerifiedEmail(GUILDID, USERID, EMAIL)
	if err != nil {
		t.Error(err)
	}
	if userID == nil {
		t.Error("User ID should not be nil")
		t.FailNow()
	}

	if *userID != USERID {
		t.Error("User ID doesn't match")
	}
	// non-existing email
	userID, err = repo.GetVerifiedEmail(GUILDID, USERID, "bad@example.com")
	if err != nil {
		t.Error(err)
	}
	if userID != nil {
		t.Error("User ID should be nil")
	}
}

func TestInsertBlockedEmails(t *testing.T) {
	err := repo.InsertEmailList(GUILDID, []string{"bob@gatekeeper.com"}, true)
	if err != nil {
		t.Error(err)
	}
	conn.Exec(context.Background(), "DELETE FROM email WHERE guild_id = $1", GUILDID)
}

func TestIsValidEmail(t *testing.T) {
	// valid
	valid, err := repo.IsEmailValid(GUILDID, USERID, "bob@example.com")
	if err != nil {
		t.Error(err)
	}
	if !valid {
		t.Error("email should be valid. email is not blocked or claimed")
	}

	// valid (email claimed by same user)
	repo.InsertVerifiedEmail(GUILDID, USERID, "bob@example.com")
	valid, err = repo.IsEmailValid(GUILDID, USERID, "bob@example.com")
	if err != nil {
		t.Error(err)
	}
	if !valid {
		t.Error("email should be valid. email is claimed by same user")
	}

	// invalid (email claimed by different user)
	valid, err = repo.IsEmailValid(GUILDID, discord.UserID(123), "bob@example.com")
	if err != nil {
		t.Error(err)
	}
	if valid {
		t.Error("email should be invalid. email is claimed by another user.")
	}

	// invalid (email is blocked)
	repo.InsertEmailList(GUILDID, []string{"alice@example.com"}, true)
	valid, err = repo.IsEmailValid(GUILDID, USERID, "alice@example.com")
	if err != nil {
		t.Error(err)
	}
	if valid {
		t.Error("email should be invalid. email is blocked")
	}

	// invalid (verified user was blocked)
	repo.InsertVerifiedEmail(GUILDID, 123, "blocked@example.com")
	repo.BanUser(GUILDID, 123)
	valid, err = repo.IsEmailValid(GUILDID, USERID, "blocked@example.com")
	if err != nil {
		t.Error(err)
	}
	if valid {
		t.Error("email should be invalid. user is blocked")
	}
}

func TestBanUser(t *testing.T) {
	repo.InsertVerifiedEmail(GUILDID, 123, "testbanuser@example.com")
	err := repo.BanUser(GUILDID, 123)
	if err != nil {
		t.Error(err)
	}
	var blocked bool
	err = conn.QueryRow(context.Background(), "SELECT blocked FROM email WHERE guild_id = $1 AND user_id = $2", GUILDID, 123).Scan(&blocked)
	if err != nil {
		t.Error(err)
	}
	if !blocked {
		t.Error("user should be blocked")
	}
}

func TestUnbanUser(t *testing.T) {
	repo.InsertVerifiedEmail(GUILDID, 456, "testbanneduser@example.com")
	repo.BanUser(GUILDID, 456)
	err := repo.UnbanUser(GUILDID, 456)
	if err != nil {
		t.Error(err)
	}
	var blocked bool
	const query = "SELECT blocked FROM email WHERE guild_id = $1 AND user_id = $2"
	err = conn.QueryRow(context.Background(), query, GUILDID, 456).Scan(&blocked)
	if err != nil {
		t.Error(err)
	}
	if blocked {
		t.Error("user should not be banned")
	}
}

func TestIsDomainValid(t *testing.T) {
	// valid
	repo.InsertDomainRole(GUILDID, &repository.DomainRole{
		Domain: "example.com",
		RoleID: 123,
	})
	valid, err := repo.IsDomainValid(GUILDID, "bob@example.com")
	if err != nil {
		t.Error(err)
	}
	if valid == "" {
		t.Error("domain should be valid")
	}

	// invalid
	valid, err = repo.IsDomainValid(GUILDID, "bob@example.net")
	if err != nil {
		t.Error(err)
	}
	if valid != "" {
		t.Error("domain should not be valid")
	}
}

func TestMain(m *testing.M) {
	var err error
	connString := "postgres://postgres:postgres@localhost:5432/"
	conn, err = pgx.Connect(context.Background(), connString)
	if err != nil {
		panic(err)
	}

	// drop database
	_, err = conn.Exec(context.Background(), "DROP DATABASE gatekeeper_test")
	if err != nil {
		panic(err)
	}
	// create database
	_, err = conn.Exec(context.Background(), "CREATE DATABASE gatekeeper_test")
	if err != nil {
		panic(err)
	}
	// connect to database
	connString += "gatekeeper_test"
	conn.Close(context.Background())

	conn, err = pgx.Connect(context.Background(), connString)
	if err != nil {
		panic(err)
	}
	content, _ := ioutil.ReadFile("../db.sql")

	_, err = conn.Exec(context.Background(), string(content))
	if err != nil {
		panic(err)
	}
	repo, _ = repository.New(connString)
	m.Run()
	repo.Close()
	conn.Close(context.Background())
}
