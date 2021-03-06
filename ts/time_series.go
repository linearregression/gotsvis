// Copyright (c) 2014 Datacratic. All rights reserved.

package ts

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"time"
)

// NewTimeSeries, not the clearest of functions, but made to be variable.
// and actually all other NewTimeSeries... call this one.
func NewTimeSeries(key string, start, end time.Time, step time.Duration, values ...float64) (*TimeSeries, error) {

	if int(step) == 0 {
		return nil, fmt.Errorf("step can't be 0")
	}

	singleValue := false
	filler := math.NaN()
	if len(values) == 1 {
		singleValue = true
		filler = values[0]
	}
	if start.IsZero() {
		start = time.Now()
	}
	knownEnd := false
	if !end.IsZero() {
		knownEnd = true
	}

	if !knownEnd && end.Before(start.Add(time.Duration(len(values))*step)) {
		end = start.Add(time.Duration(len(values)) * step)
	}

	ts := &TimeSeries{
		key:    key,
		start:  start,
		step:   step,
		filler: filler,
	}

	if ts.start.After(end) {
		return nil, fmt.Errorf("start time %v can't be after end %v time", start, end)
	}
	if !ts.start.Equal(end) {
		if int64(ts.step) == 0 {
			return nil, fmt.Errorf("step size can't be 0 if start != end time")
		}
	}

	size := int(end.Sub(ts.start) / ts.step)
	ts.data = make([]float64, size)

	for i, _ := range ts.data {
		if singleValue {
			ts.data[i] = filler
		} else if i < len(values) {
			ts.data[i] = values[i]
		} else {
			break
		}
	}

	return ts, nil
}

func NewTimeSeriesOfTimeRange(key string, start, end time.Time, step time.Duration, filler float64) (*TimeSeries, error) {
	return NewTimeSeries(key, start, end.Add(step), step, filler)
}

func NewTimeSeriesOfLength(key string, start time.Time, step time.Duration, length int, filler float64) (*TimeSeries, error) {
	return NewTimeSeries(key, start, start.Add(time.Duration(length)*step), step, filler)
}

func NewTimeSeriesOfData(key string, start time.Time, step time.Duration, data []float64) (*TimeSeries, error) {
	return NewTimeSeries(key, start, time.Time{}, step, data...)
}

type TimeSeries struct {
	key    string
	start  time.Time
	step   time.Duration
	data   []float64
	filler float64
}

func (ts *TimeSeries) Key() string {
	return ts.key
}
func (ts *TimeSeries) SetKey(key string) {
	ts.key = key
}
func (ts *TimeSeries) Start() time.Time {
	return ts.start
}
func (ts *TimeSeries) End() time.Time {
	return ts.start.Add(time.Duration(len(ts.data)) * ts.step)
}
func (ts *TimeSeries) Step() time.Duration {
	return ts.step
}
func (ts *TimeSeries) Data() []float64 {
	data := make([]float64, len(ts.data))
	for i, v := range ts.data {
		data[i] = v
	}
	return data
}
func (ts *TimeSeries) Copy() *TimeSeries {
	nts := &TimeSeries{
		key:    ts.key,
		start:  ts.start,
		step:   ts.step,
		data:   ts.Data(),
		filler: ts.filler,
	}
	return nts
}

func (ts *TimeSeries) ExtendTo(t time.Time) {
	end := ts.End()
	if t.Before(end) {
		return
	}
	t = t.Add(ts.step)
	points := t.Sub(end) / ts.step

	for i := 0; i < int(points); i++ {
		ts.data = append(ts.data, ts.filler)
	}
}
func (ts *TimeSeries) ExtendBy(d time.Duration) {
	points := d / ts.step
	for i := 0; i < int(points); i++ {
		ts.data = append(ts.data, ts.filler)
	}
}
func (ts *TimeSeries) ExtendWith(data ...float64) {
	ts.data = append(ts.data, data...)
}

func (ts *TimeSeries) index(t time.Time) int {
	if t.Before(ts.start) {
		return -1
	}

	end := ts.start.Add(time.Duration(len(ts.data)-1) * ts.step)
	if t.After(end) {
		return -1
	}

	distance := t.Sub(ts.start)
	index := distance / ts.step
	return int(index)
}

func (ts *TimeSeries) GetAt(t time.Time) (float64, bool) {
	index := ts.index(t)
	if index == -1 {
		return math.NaN(), false
	}
	return ts.data[index], true
}

func (ts *TimeSeries) SetAt(t time.Time, value float64) bool {
	index := ts.index(t)
	if index == -1 {
		return false
	}

	ts.data[index] = value
	return true
}

func (ts *TimeSeries) IsEqualStep(other *TimeSeries) bool {
	return ts.step == other.step
}

func (ts *TimeSeries) Transform(transform Transform) *TimeSeries {
	tts := ts.Copy()
	tts.key = transform.Name() + "(" + ts.key + ")"

	for i, v := range tts.data {
		tts.data[i] = transform.Transform(v)
	}

	return tts
}

func (ts TimeSeries) String() string {
	s := bytes.NewBufferString("")
	s.WriteString(ts.key)

	s.WriteString(" Start: ")
	s.WriteString(ts.start.String())

	s.WriteString(" End: ")
	s.WriteString(ts.End().String())

	s.WriteString(" Step: ")
	s.WriteString(ts.step.String())

	s.WriteString(" Length: ")
	s.WriteString(strconv.Itoa(len(ts.data)))

	s.WriteString(" ")

	for _, v := range ts.data {
		s.WriteString(strconv.FormatFloat(v, 'f', 2, 64))
		s.WriteByte(',')
	}
	if s.Len() > 0 {
		s.Truncate(s.Len() - 1)
	}
	return s.String()
}

type Transform interface {
	Name() string
	Transform(float64) float64
}

func (ts *TimeSeries) Iterator() *Iterator {
	return &Iterator{
		cursor: ts.start,
		series: ts,
	}
}

type Iterator struct {
	cursor time.Time
	series *TimeSeries
}

func (it *Iterator) Next() (val float64, ok bool) {
	val, ok = it.series.GetAt(it.cursor)
	it.cursor = it.cursor.Add(it.series.step)
	return
}

func (it *Iterator) Last() (val float64, ok bool) {
	it.cursor = it.series.End().Add(-it.series.step)
	val, ok = it.series.GetAt(it.cursor)
	return
}

type IteratorTimeValue struct {
	Iterator
}

func (ts *TimeSeries) IteratorTimeValue() *IteratorTimeValue {
	return &IteratorTimeValue{Iterator{
		cursor: ts.start,
		series: ts,
	}}
}

func (it *IteratorTimeValue) Next() (t time.Time, val float64, ok bool) {
	t = it.cursor
	val, ok = it.series.GetAt(it.cursor)
	it.cursor = it.cursor.Add(it.series.step)
	return
}

func (it *IteratorTimeValue) Last() (t time.Time, val float64, ok bool) {
	it.cursor = it.series.End().Add(-it.series.step)
	t = it.cursor
	val, ok = it.series.GetAt(it.cursor)
	return
}
