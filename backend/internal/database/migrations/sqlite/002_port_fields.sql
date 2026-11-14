-- ADD COLUMN directives check column existence before executing ALTER TABLE.
ADD COLUMN users revision bigint DEFAULT 1;
ADD COLUMN passkeys credential_data text;
ADD COLUMN remnawave_settings title varchar(128) DEFAULT 'Remnawave';
ADD COLUMN remnawave_settings logo_url varchar(255);
ADD COLUMN remnawave_settings is_login_allowed boolean DEFAULT true;
ADD COLUMN remnawave_settings is_register_allowed boolean DEFAULT false;
ADD COLUMN remnawave_settings password_auth_enabled boolean DEFAULT true;
