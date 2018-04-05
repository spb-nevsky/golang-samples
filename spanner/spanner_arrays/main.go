// Copyright 2017 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// Sample spanner_arrays is a basic program that queries Google's Cloud Spanner
// and returns an array within the results.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"regexp"
	"strings"

	"cloud.google.com/go/spanner"
	database "cloud.google.com/go/spanner/admin/database/apiv1"
	"google.golang.org/api/iterator"
	adminpb "google.golang.org/genproto/googleapis/spanner/admin/database/v1"
)

// Country describes a country and the cities inside it
type Country struct {
	Name   string
	Cities []spanner.NullString
}

func main() {
	ctx := context.Background()

	dsn := flag.String("database", "projects/your-project-id/instances/your-instance-id/databases/your-database-id", "Cloud Spanner database name")
	flag.Parse()

	// Connect to the Spanner Admin API
	admin, err := database.NewDatabaseAdminClient(ctx)
	if err != nil {
		log.Fatalf("failed to create database admin client: %v", err)
	}
	defer admin.Close()

	err = createDatabase(ctx, admin, *dsn)
	if err != nil {
		log.Fatalf("failed to create database: %v", err)
	}
	defer removeDatabase(ctx, admin, *dsn)

	// Connect to database
	client, err := spanner.NewClient(ctx, *dsn)
	if err != nil {
		log.Fatalf("Failed to create client %v", err)
	}
	defer client.Close()

	err = loadPresets(ctx, client)
	if err != nil {
		log.Fatalf("failed to load preset data: %v", err)
	}

	it := client.Single().Query(ctx, spanner.NewStatement(`
		SELECT a.Name AS Name, ARRAY(
			SELECT b.Name FROM Cities b WHERE a.CountryId = b.CountryId
		) AS Cities FROM Countries a
	`))
	defer it.Stop()

	for {
		row, err := it.Next()
		if err == iterator.Done {
			return
		}
		if err != nil {
			log.Fatalf("failed to read results: %v", err)
		}

		var country Country
		err = row.ToStruct(&country)
		if err != nil {
			log.Fatalf("failed to read row into Country struct: %s", err)
		}

		var cities []string
		for _, c := range country.Cities {
			cities = append(cities, c.String())
		}

		log.Printf("%s: %s", country.Name, strings.Join(cities, ", "))
	}
}

func loadPresets(ctx context.Context, db *spanner.Client) error {
	mx := []*spanner.Mutation{
		spanner.InsertMap("Countries", map[string]interface{}{
			"CountryId": 49,
			"Name":      "Germany",
		}),
		spanner.InsertMap("Cities", map[string]interface{}{
			"CountryId": 49,
			"CityId":    100,
			"Name":      "Berlin",
		}),
		spanner.InsertMap("Cities", map[string]interface{}{
			"CountryId": 49,
			"CityId":    101,
			"Name":      "Hamburg",
		}),
		spanner.InsertMap("Cities", map[string]interface{}{
			"CountryId": 49,
			"CityId":    102,
			"Name":      "Dresden",
		}),
		spanner.InsertMap("Countries", map[string]interface{}{
			"CountryId": 44,
			"Name":      "United Kingdom",
		}),
		spanner.InsertMap("Cities", map[string]interface{}{
			"CountryId": 44,
			"CityId":    200,
			"Name":      "London",
		}),
		spanner.InsertMap("Cities", map[string]interface{}{
			"CountryId": 44,
			"CityId":    201,
			"Name":      "Liverpool",
		}),
		spanner.InsertMap("Cities", map[string]interface{}{
			"CountryId": 44,
			"CityId":    202,
			"Name":      "Bristol",
		}),
		spanner.InsertMap("Cities", map[string]interface{}{
			"CountryId": 44,
			"CityId":    203,
			"Name":      "Newcastle",
		}),
	}

	_, err := db.Apply(ctx, mx)
	return err
}

func createDatabase(ctx context.Context, adminClient *database.DatabaseAdminClient, db string) error {
	matches := regexp.MustCompile("^(.*)/databases/(.*)$").FindStringSubmatch(db)
	if matches == nil || len(matches) != 3 {
		log.Fatalf("Invalid database id %s", db)
	}

	op, err := adminClient.CreateDatabase(ctx, &adminpb.CreateDatabaseRequest{
		Parent:          matches[1],
		CreateStatement: fmt.Sprintf("CREATE DATABASE `%s`", matches[2]),
		ExtraStatements: []string{
			`CREATE TABLE Countries (
				CountryId 	INT64 NOT NULL,
				Name   		STRING(1024) NOT NULL
			) PRIMARY KEY (CountryId)`,
			`CREATE TABLE Cities (
				CountryId	INT64 NOT NULL,
				CityId		INT64 NOT NULL,
				Name			STRING(MAX),
			) PRIMARY KEY (CountryId, CityId),
			INTERLEAVE IN PARENT Countries ON DELETE CASCADE`,
		},
	})
	if err != nil {
		return err
	}
	if _, err := op.Wait(ctx); err == nil {
		log.Printf("Created database [%s]", db)
	}
	return err
}

func removeDatabase(ctx context.Context, adminClient *database.DatabaseAdminClient, db string) error {
	err := adminClient.DropDatabase(ctx, &adminpb.DropDatabaseRequest{Database: db})
	if err != nil {
		log.Fatalf("Failed to remove database [%s]: %v", db, err)
	}
	log.Printf("Removed database [%s]", db)
	return nil
}
