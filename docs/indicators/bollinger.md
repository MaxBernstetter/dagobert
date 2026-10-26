# Bollinger Bands

The middle band is a `MovingAverage` with `method` `sma`. It is not a second average.

```json
{"type": "Bollinger", "time_frame_hr": 3, "width": 2}
```

`time_frame_hr` is a positive multiple of 3. The bands are `price`.
