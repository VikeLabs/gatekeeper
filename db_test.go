package main

import (
	"database/sql"
	"math"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestDBSnowflake(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Error("failed to open db:", err)
	}

	_, err = db.Exec(`CREATE TABLE test (
		id BIGINT NOT NULL,
		snowflake BIGINT NOT NULL,
		PRIMARY KEY (id)
	);`)
	if err != nil {
		t.Error(err)
	}

	id := int64(0)

	T := func(expected DBSnowflake) {
		_, err = db.Exec("INSERT INTO test (id, snowflake) VALUES ($1,$2)", id, expected)
		if err != nil {
			t.Error(err)
		}

		row := db.QueryRow("SELECT snowflake FROM test WHERE id = $1", id)
		var actual DBSnowflake
		err = row.Scan(&actual)
		if err != nil {
			t.Error(err)
		}

		if expected != actual {
			t.Errorf("expected %v, got %v", expected, actual)
		}

		id++
	}

	T(1)
	T(2)
	T(3)
	T(100)
	T(1234567)
	T(math.MaxInt8 >> 1)
	T(math.MaxInt8)
	T(math.MaxInt8 + 1)
	T(math.MaxInt8 + 123)
	T(math.MaxInt16 >> 1)
	T(math.MaxInt16)
	T(math.MaxInt16 + 1)
	T(math.MaxInt16 + 123)
	T(math.MaxInt32 >> 1)
	T(math.MaxInt32)
	T(math.MaxInt32 + 1)
	T(math.MaxInt32 + 123)
	T(math.MaxInt64 >> 1)
	T(math.MaxInt64)
	T(math.MaxUint8 >> 1)
	T(math.MaxUint8)
	T(math.MaxUint8 + 1)
	T(math.MaxUint8 + 123)
	T(math.MaxUint16 >> 1)
	T(math.MaxUint16)
	T(math.MaxUint16 + 1)
	T(math.MaxUint16 + 123)
	T(math.MaxUint32 >> 1)
	T(math.MaxUint32)
	T(math.MaxUint32 + 1)
	T(math.MaxUint32 + 123)
	T(math.MaxUint64 >> 1)
	T(math.MaxUint64)

	err = db.Close()
	if err != nil {
		t.Error(err)
	}
}

func FuzzDBSnowflake(f *testing.F) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		f.Error("failed to open db:", err)
	}

	_, err = db.Exec(`CREATE TABLE test (
		id BIGINT NOT NULL,
		snowflake BIGINT NOT NULL,
		PRIMARY KEY (id)
	);`)
	if err != nil {
		f.Error(err)
	}

	seed := []uint64{
		1,
		2,
		3,
		100,
		1234567,
		math.MaxInt8 >> 1,
		math.MaxInt8,
		math.MaxInt8 + 1,
		math.MaxInt8 + 123,
		math.MaxInt16 >> 1,
		math.MaxInt16,
		math.MaxInt16 + 1,
		math.MaxInt16 + 123,
		math.MaxInt32 >> 1,
		math.MaxInt32,
		math.MaxInt32 + 1,
		math.MaxInt32 + 123,
		math.MaxInt64 >> 1,
		math.MaxInt64,
		math.MaxUint8 >> 1,
		math.MaxUint8,
		math.MaxUint8 + 1,
		math.MaxUint8 + 123,
		math.MaxUint16 >> 1,
		math.MaxUint16,
		math.MaxUint16 + 1,
		math.MaxUint16 + 123,
		math.MaxUint32 >> 1,
		math.MaxUint32,
		math.MaxUint32 + 1,
		math.MaxUint32 + 123,
		math.MaxUint64 >> 1,
		math.MaxUint64,
	}

	for _, tc := range seed {
		f.Add(tc)
	}

	id := int64(0)

	f.Fuzz(func(t *testing.T, input uint64) {
		expected := DBSnowflake(input)
		_, err = db.Exec("INSERT INTO test (id, snowflake) VALUES ($1,$2)", id, expected)
		if err != nil {
			t.Error(err)
		}

		row := db.QueryRow("SELECT snowflake FROM test WHERE id = $1", id)
		var actual DBSnowflake
		err = row.Scan(&actual)
		if err != nil {
			t.Error(err)
		}

		if expected != actual {
			t.Errorf("expected %v, got %v", expected, actual)
		}

		id++
	})

	err = db.Close()
	if err != nil {
		f.Error(err)
	}
}
