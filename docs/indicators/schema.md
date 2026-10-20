# Indicator schema

A technical indicator is a parameter object, not a column name. Two uses that differ only by a time span share one definition.

```json
{"type": "MovingAverage", "method": "sma", "time_frame_hr": 3}
```

`method` is `sma` or `ema`. `time_frame_hr` is a positive multiple of 3. The same object with another `time_frame_hr` is another result of this definition.

The object is stored in a canonical form: keys sorted, no extra keys, numbers as integers. Two objects that canonicalize to the same text are one node in the calculation graph.

Each type declares the unit of the value it produces.

| Unit | Meaning | Compared with |
|---|---|---|
| `price` | Same unit as the bar close | another `price` |
| `amount` | A quantity, such as base-asset volume | another `amount` of the same quantity |
| `ratio` | A change relative to a base, not a price | another `ratio` |
| `index` | A bounded scale, such as RSI | another `index` of the same type |

A playbook may compare two results only when the unit matches. The process does not rescale a price into an index to make a comparison possible.
