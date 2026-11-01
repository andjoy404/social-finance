-- Migration 046: Enforce unique multi-month IPL issuance per calendar year per RT
CREATE UNIQUE INDEX IF NOT EXISTS idx_dues_rt_multi_month_year
ON dues (rt_id, year)
WHERE due_type = 'multi_month';

