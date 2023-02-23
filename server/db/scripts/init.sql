CREATE TABLE "users" (
    "id" varchar(255) UNIQUE PRIMARY KEY,
    "email" varchar(127),
    "calendar_token" text,
    "settings" text,
    "allow_notify" varchar(1) DEFAULT 'Y',
    "created_at" timestamptz,
    "updated_at" timestamptz,
    "tcc_state" int DEFAULT 0
);

CREATE TABLE "lookups" (
    "id" BIGSERIAL PRIMARY KEY,
    "user_id" varchar(255) NOT NULL,
    "key" varchar(255) NOT NULL,
    "value" text NOT NULL,
    "created_at" timestamptz,
    "updated_at" timestamptz,
    "tcc_state" int DEFAULT 0
);

CREATE TABLE "connect_states" (
    "user_id" varchar(255) UNIQUE NOT NULL,
    "state" varchar(255) UNIQUE NOT NULL
);

CREATE INDEX "idx_users_id" ON "users" ("id");

CREATE INDEX "idx_users_email" ON "users" ("email");

CREATE INDEX "idx_users_allow_notify" ON "users" ("allow_notify");

CREATE INDEX "idx_lookups_user_id_key" ON "lookups" ("user_id", "key");

CREATE INDEX "idx_connect_state_user_id" ON "connect_states" ("user_id");

ALTER TABLE
    "lookups"
ADD
    FOREIGN KEY ("user_id") REFERENCES "users" ("id");