// Package xgift holds generic building blocks on top of gift
// (github.com/worldiety/gift) that are not specific to this application:
// asynchronous resources, in-memory pictures, QR codes, grids, a PIN pad, a
// stepper, chips, touch friendly multi selection for galleries and a hidden
// tap gesture.
//
// Everything here is written so that it could move into gift itself: no
// imports from the application, English documentation, and the same
// conventions as gift's ui package (value views, explicit keys, no hidden
// global state except the one App handle documented at [Install]).
package xgift
