ALTER TABLE iterations DROP CONSTRAINT IF EXISTS iterations_decisions_object_check;
ALTER TABLE iterations DROP COLUMN IF EXISTS decisions;
ALTER TABLE iterations DROP COLUMN IF EXISTS branch_id;
DROP TABLE IF EXISTS branches;