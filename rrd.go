package main

import (
	"github.com/ziutek/rrd"
	"fmt"
	"time"
)

type Rrd struct {
	path        string
	creator    *rrd.Creator
	updater    *rrd.Updater
	grapher    *rrd.Grapher
}

func NewRrd(filename string) (*Rrd) {
	r := Rrd{
		path: filename,
		updater: rrd.NewUpdater(filename),
		grapher: rrd.NewGrapher(),
	}
	r.creator = rrd.NewCreator(r.path, time.Now(), 10)
	return &r
}

func (r *Rrd) AddStandardRRAs() {
	r.creator.RRA("AVERAGE", "0.5", "3",  "400000")
	r.creator.RRA("MAX",     "0.5", "60", "400000")
}

// Used to create an RRD, only call once in the life of an RRD
func (r *Rrd) Creator() (*rrd.Creator) {
	return r.creator
}

// Updates the data in the RRD
func (r *Rrd) Updater() (*rrd.Updater) {
	return r.updater
}

// Used to describe and configure the graph
func (r *Rrd) Grapher() (*rrd.Grapher) {
	return r.grapher
}

// Creates and saves the graph
func (r *Rrd) SaveGraph(start, end time.Time) (error) {
	_, err := r.grapher.SaveGraph(r.path, start, end)
	return err
}

// Popular pallet for graphing
func colorStr(count int) string {
	TABLEAU_20 := [][]int{
		{ 31, 119, 180}, {174, 199, 232}, {255, 127,  14}, {255, 187, 120},
		{ 44, 160,  44}, {152, 223, 138}, {214,  39,  40}, {255, 152, 150},
		{148, 103, 189}, {197, 176, 213}, {140,  86,  75}, {196, 156, 148},
		{227, 119, 194}, {247, 182, 210}, {127, 127, 127}, {199, 199, 199},
		{188, 189,  34}, {219, 219, 141}, { 23, 190, 207}, {158, 218, 229}}

	return fmt.Sprintf("%02x%02x%02x", TABLEAU_20[count%20][0],
		TABLEAU_20[count%20][1], TABLEAU_20[count%20][2])
}
