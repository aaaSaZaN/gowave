-- The job claim walks unfinished jobs in queue order and skips targets that
-- already have a running job; without these it sorts the whole table.
CREATE INDEX IF NOT EXISTS `idx_jobs_ready` ON `jobs`(`created_at`, `id`) WHERE `state` = 'pending' OR `state` = 'running';
CREATE INDEX IF NOT EXISTS `idx_jobs_running_target` ON `jobs`(`target`) WHERE `state` = 'running';
