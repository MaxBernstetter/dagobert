
import * as React from "react";
import { CalendarIcon } from "lucide-react";
import type { Matcher } from "react-day-picker";
import { cn } from "@/lib/utils";
import { buttonVariants } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import {
  backtestDateInputToPickerDate,
  backtestPickerDateToDateInput,
  formatBacktestDateLabel,
} from "@/lib/backtest-shared";

type Props = {
  label: string;
  value: string;
  onChange: (dateInput: string) => void;
  min?: string;
  max?: string;
  id?: string;
};

function BacktestDatePickerInner({ label, value, onChange, min, max, id }: Props) {
  const [open, setOpen] = React.useState(false);

  const selected = React.useMemo(() => backtestDateInputToPickerDate(value), [value]);
  const [month, setMonth] = React.useState<Date>(() => selected ?? new Date());

  React.useEffect(() => {
    const d = backtestDateInputToPickerDate(value);
    if (d) setMonth(d);
  }, [value]);

  const disabled = React.useMemo((): Matcher[] | undefined => {
    const matchers: Matcher[] = [];
    const minDate = min ? backtestDateInputToPickerDate(min) : undefined;
    const maxDate = max ? backtestDateInputToPickerDate(max) : undefined;
    if (minDate) matchers.push({ before: minDate });
    if (maxDate) matchers.push({ after: maxDate });
    return matchers.length > 0 ? matchers : undefined;
  }, [min, max]);

  const { startMonth, endMonth } = React.useMemo(() => {
    const minDate = min ? backtestDateInputToPickerDate(min) : undefined;
    const maxDate = max ? backtestDateInputToPickerDate(max) : undefined;
    const anchor = selected ?? new Date();
    const start = minDate ?? new Date(anchor.getFullYear() - 10, 0, 1);
    const end = maxDate ?? new Date(anchor.getFullYear() + 2, 11, 31);
    return { startMonth: start, endMonth: end };
  }, [min, max, selected]);

  const handleOpenChange = React.useCallback(
    (next: boolean) => {
      if (next) {
        const d = backtestDateInputToPickerDate(value);
        if (d) setMonth(d);
      }
      setOpen(next);
    },
    [value],
  );

  return (
    <div className="grid gap-1">
      <span className="text-muted-foreground" id={id ? `${id}-label` : undefined}>
        {label}
      </span>
      <Popover open={open} onOpenChange={handleOpenChange}>
        <PopoverTrigger
          type="button"
          id={id}
          aria-labelledby={id ? `${id}-label` : undefined}
          className={cn(
            buttonVariants({ variant: "outline" }),
            "h-auto w-full justify-start gap-2 px-2 py-1.5 font-normal",
            !value && "text-muted-foreground"
          )}
        >
          <CalendarIcon className="size-4 shrink-0 opacity-70" />
          <span className="truncate">{formatBacktestDateLabel(value)}</span>
        </PopoverTrigger>
        <PopoverContent className="w-auto p-0" align="start" side="bottom">
          {open ? (
            <Calendar
              mode="single"
              captionLayout="dropdown"
              month={month}
              onMonthChange={setMonth}
              startMonth={startMonth}
              endMonth={endMonth}
              selected={selected}
              disabled={disabled}
              onSelect={(day) => {
                if (!day) return;
                onChange(backtestPickerDateToDateInput(day));
                setOpen(false);
              }}
            />
          ) : null}
        </PopoverContent>
      </Popover>
    </div>
  );
}

/**
 * Memoized: the experiments panel re-renders on its polling tick. A picker whose props did not change
 * keeps its open calendar (and the month and year dropdowns in it) instead of re-rendering.
 */
export const BacktestDatePicker = React.memo(BacktestDatePickerInner);
