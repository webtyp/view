# Tests: webtyp.com/view

| Test | File | Setup → call → expected |
|---|---|---|
| `TestNotify_AutoSizesToWords` | `tests/notify_test.go` | `Auto().Millis("ok")` → `4000`; 40 words → `10000`; `Auto().IsAuto()` → true |
| `TestNotify_ForAndPersistent` | `tests/notify_test.go` | `For(1500).Millis("x")` → `1500`; `Persistent().HasTimer()` → false, `.Millis("x")` → `0` |
| `TestDemo_ValidateNamesFirstMissingField` | `tests/demo_test.go` | table: zero `Demo` → `ErrDemoNoLabel`; Label only → `ErrDemoNoTimezone`; + Timezone → `ErrDemoNoModules`; + one fake `crud.Module` → `ErrDemoNoScreens`; complete → `nil` |
