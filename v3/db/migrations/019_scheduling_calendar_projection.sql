ALTER TABLE app.scheduling_bookings
  ADD COLUMN IF NOT EXISTS meet_requested boolean NOT NULL DEFAULT false;

ALTER TABLE app.scheduling_waitlist
  ADD COLUMN IF NOT EXISTS meet_requested boolean NOT NULL DEFAULT false;
