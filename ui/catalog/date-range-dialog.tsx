import React, { useRef, useState } from "react";
import {
  inclusiveRange,
  presetRange,
  presets,
  rangeDays,
  rangeLabel,
  shiftDay,
  type DateRange,
} from "./analytics-data";

export function DateRangeDialog({
  value,
  onApply,
}: {
  value: DateRange;
  onApply: (range: DateRange) => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [start, setStart] = useState(value.from),
    [end, setEnd] = useState(shiftDay(value.to, -1));
  const [preset, setPreset] = useState("Custom range"),
    [error, setError] = useState("");
  let draft: DateRange | undefined;
  try {
    draft = inclusiveRange(start, end);
  } catch {
    /* Show validation when applying. */
  }
  const close = () => dialog.current?.close();
  return (
    <>
      <button
        type="button"
        className="tlink analytics-date-trigger"
        onClick={() => {
          setStart(value.from);
          setEnd(shiftDay(value.to, -1));
          setPreset("Custom range");
          setError("");
          dialog.current?.showModal();
        }}
      >
        {rangeLabel(value)} <span aria-hidden="true">▾</span>
      </button>
      <dialog
        ref={dialog}
        className="analytics-date-dialog"
        aria-labelledby="analytics-date-title"
      >
        <form
          onSubmit={(event) => {
            event.preventDefault();
            try {
              const range = inclusiveRange(start, end);
              onApply(range);
              close();
            } catch (e) {
              setError((e as Error).message);
            }
          }}
        >
          <div className="analytics-dialog-head">
            <h2 id="analytics-date-title">Date range</h2>
            <button type="button" className="tlink" onClick={close}>
              Close
            </button>
          </div>
          <div className="seg small analytics-presets">
            {[...presets, "Custom range"].map((name) => (
              <button
                key={name}
                type="button"
                aria-pressed={preset === name}
                onClick={() => {
                  setPreset(name);
                  setError("");
                  if (name !== "Custom range") {
                    const range = presetRange(name as (typeof presets)[number]);
                    setStart(range.from);
                    setEnd(shiftDay(range.to, -1));
                  }
                }}
              >
                {name}
              </button>
            ))}
          </div>
          <div className="analytics-date-fields">
            <label className="field">
              Start date
              <input
                type="date"
                required
                value={start}
                onChange={(e) => {
                  setStart(e.target.value);
                  setPreset("Custom range");
                  setError("");
                }}
              />
            </label>
            <label className="field">
              End date (included)
              <input
                type="date"
                required
                value={end}
                onChange={(e) => {
                  setEnd(e.target.value);
                  setPreset("Custom range");
                  setError("");
                }}
              />
            </label>
          </div>
          <p className="small muted">
            Use a preset or open either date field’s calendar.
            <br />
            All dates use UTC. Choose up to 90 days; the end date is included.
          </p>
          {error && (
            <p className="warn" role="alert">
              {error}
            </p>
          )}
          <div className="analytics-dialog-actions">
            <span className="small muted">
              {draft
                ? `${rangeLabel(draft)} · ${rangeDays(draft).length} days`
                : "Choose a valid range"}
            </span>
            <div className="actions">
              <button type="button" className="tlink" onClick={close}>
                Cancel
              </button>
              <button className="btn btn-ink" type="submit">
                Apply range
              </button>
            </div>
          </div>
        </form>
      </dialog>
    </>
  );
}
