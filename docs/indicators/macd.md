# MACD

Depends on two `MovingAverage` results with `method` `ema`. Does not compute those averages again.

```json
{"type": "Macd", "fast_hr": 12, "slow_hr": 26, "signal_hr": 9}
```

`fast_hr`, `slow_hr`, and `signal_hr` are positive multiples of 3. The line and the signal are `price`. The histogram is `price`, the difference of those two, not a second MACD.
