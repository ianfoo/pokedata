package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/gorilla/mux"
	"github.com/mtslzr/pokeapi-go"
	"github.com/mtslzr/pokeapi-go/structs"
	"github.com/sirupsen/logrus"
)

func main() {
	if err := run(); err != nil {
		logrus.WithError(err).Fatal("critical error")
	}
}

func run() error {
	logrus.Info("starting")
	defer logrus.Info("exiting")

	resource, err := pokeapi.Resource("pokemon")
	if err != nil {
		return err
	}
	var (
		pokemonDefs = make(map[string]structs.Pokemon)
		names       []string
	)
	for _, result := range resource.Results {
		pokemonDefs[result.Name] = structs.Pokemon{}
		names = append(names, result.Name)
	}
	logrus.WithFields(logrus.Fields{
		"count": len(resource.Results),
		"names": strings.Join(names, ", "),
	}).Info("retrieved pokemon names")
	var (
		r    = mux.NewRouter()
		lock sync.RWMutex
	)
	r.HandleFunc("/pokemon/{pokemon}", mkPokemonHandler(pokemonDefs, &lock)).Methods("GET")
	return http.ListenAndServe(":2020", r)
}

func mkPokemonHandler(pokemonDefs map[string]structs.Pokemon, lock *sync.RWMutex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			vars        = mux.Vars(r)
			pokemon, ok = vars["pokemon"]
		)
		if pokemon == "" {
			http.Error(w, "must provide a pokemon name!", http.StatusBadRequest)
			return
		}
		lock.RLock()
		pokemonDef, ok := pokemonDefs[pokemon]
		lock.RUnlock()
		if !ok {
			http.Error(w, fmt.Sprintf("%q is not a known pokemon", pokemon), http.StatusBadRequest)
			return
		}
		if !reflect.DeepEqual(pokemonDef, structs.Pokemon{}) {
			if err := json.NewEncoder(w).Encode(pokemonDef); err != nil {
				http.Error(w, "error rendering %q record", http.StatusInternalServerError)
				logrus.WithError(err).WithField("pokemon", pokemon).Error("JSON render error")
				return
			}
			logrus.WithField("pokemon", pokemon).Info("served from cache")
		}
		pokemonDef, err := pokeapi.Pokemon(pokemon)
		if err != nil {
			http.Error(w, fmt.Sprintf("error fetching pokemon %q record", pokemon), http.StatusInternalServerError)
			logrus.WithError(err).WithField("pokemon", pokemon).Error("error fetching pokemon record")
			return
		}
		lock.Lock()
		pokemonDefs[pokemon] = pokemonDef
		lock.Unlock()
		logrus.WithField("pokemon", pokemon).Info("added to cache")
		if err := json.NewEncoder(w).Encode(pokemonDef); err != nil {
			http.Error(w, "error rendering %q record", http.StatusInternalServerError)
			logrus.WithError(err).WithField("pokemon", pokemon).Error("JSON render error")
			return
		}
	}
}
