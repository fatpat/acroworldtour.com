package service

import "testing"

func TestBusSubscribeAll(t *testing.T) {
	b := NewBus()
	all, cancel := b.SubscribeAll()
	b.Publish(Event{Topic: "competition:a", Kind: KindFlight})
	b.Publish(Event{Topic: "season:b", Kind: KindRun})
	if e := <-all; e.Topic != "competition:a" {
		t.Errorf("first event %v", e)
	}
	if e := <-all; e.Topic != "season:b" {
		t.Errorf("second event %v", e)
	}
	cancel()
	cancel()
	if _, ok := <-all; ok {
		t.Error("channel still open after cancel")
	}
	b.Publish(Event{Topic: "competition:a"}) // must not panic on the closed subscriber
}
