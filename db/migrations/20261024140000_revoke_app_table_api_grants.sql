-- App tables are for the migration role / table owner only.
-- Supabase grants public tables to anon and authenticated for PostgREST; revoke those.
REVOKE ALL ON TABLE public.experiment FROM anon, authenticated, PUBLIC;
REVOKE ALL ON TABLE public.experiment_instance FROM anon, authenticated, PUBLIC;
REVOKE ALL ON TABLE public.experiment_instance_period_stat FROM anon, authenticated, PUBLIC;
REVOKE ALL ON TABLE public.experiment_instance_action FROM anon, authenticated, PUBLIC;
REVOKE ALL ON TABLE public.experiment_instance_fill FROM anon, authenticated, PUBLIC;
