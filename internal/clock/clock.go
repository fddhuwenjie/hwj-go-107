// Package clock 提供可注入时钟，生产环境使用真实时钟，测试使用假时钟。
package clock

import "time"

// Clock 时钟接口，所有业务时间获取均通过该接口，保证测试可控。
type Clock interface {
	// Now 返回当前时间。
	Now() time.Time
}

// Real 真实时钟。
type Real struct{}

// Now 返回系统当前时间。
func (Real) Now() time.Time { return time.Now() }

// Fake 假时钟，用于测试中精确控制时间推进。
type Fake struct {
	now time.Time
}

// NewFake 创建假时钟。
func NewFake(t time.Time) *Fake { return &Fake{now: t} }

// Now 返回假时钟当前时间。
func (f *Fake) Now() time.Time { return f.now }

// Advance 将假时钟推进 d。
func (f *Fake) Advance(d time.Duration) { f.now = f.now.Add(d) }

// Set 直接设置假时钟时间。
func (f *Fake) Set(t time.Time) { f.now = t }
