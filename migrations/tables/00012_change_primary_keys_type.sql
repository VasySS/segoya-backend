-- +goose Up
-- +goose StatementBegin

-- Convert user_info (referenced by many tables)
ALTER TABLE user_info ADD COLUMN new_id UUID DEFAULT uuidv7();

-- Update foreign keys pointing to user_info
ALTER TABLE user_oauth ADD COLUMN new_user_id UUID;
UPDATE user_oauth AS uo SET new_user_id = ui.new_id FROM user_info AS ui WHERE uo.user_id = ui.id;

ALTER TABLE singleplayer_game ADD COLUMN new_user_id UUID;
UPDATE singleplayer_game AS sg SET new_user_id = ui.new_id FROM user_info AS ui WHERE sg.user_id = ui.id;

ALTER TABLE multiplayer_game ADD COLUMN new_creator_id UUID;
UPDATE multiplayer_game AS mg SET new_creator_id = ui.new_id FROM user_info AS ui WHERE mg.creator_id = ui.id;

ALTER TABLE multiplayer_game_user ADD COLUMN new_user_id UUID;
UPDATE multiplayer_game_user AS mgu SET new_user_id = ui.new_id FROM user_info AS ui WHERE mgu.user_id = ui.id;

ALTER TABLE multiplayer_round_user ADD COLUMN new_user_id UUID;
UPDATE multiplayer_round_user AS mru SET new_user_id = ui.new_id FROM user_info AS ui WHERE mru.user_id = ui.id;

-- Drop old constraints and columns for user_info
ALTER TABLE user_oauth DROP CONSTRAINT user_oauth_user_id_fkey;
ALTER TABLE singleplayer_game DROP CONSTRAINT singleplayer_game_user_id_fkey;
ALTER TABLE multiplayer_game DROP CONSTRAINT multiplayer_game_creator_id_fkey;
ALTER TABLE multiplayer_game_user DROP CONSTRAINT multiplayer_game_user_user_id_fkey;
ALTER TABLE multiplayer_round_user DROP CONSTRAINT multiplayer_round_user_user_id_fkey;

ALTER TABLE user_info DROP CONSTRAINT user_info_pkey CASCADE;
ALTER TABLE user_info DROP COLUMN id;
ALTER TABLE user_info RENAME COLUMN new_id TO id;
ALTER TABLE user_info ADD PRIMARY KEY (id);

-- Finalize user_info foreign keys
ALTER TABLE user_oauth DROP COLUMN user_id;
ALTER TABLE user_oauth RENAME COLUMN new_user_id TO user_id;
ALTER TABLE user_oauth ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE user_oauth ADD FOREIGN KEY (user_id) REFERENCES user_info(id);

ALTER TABLE singleplayer_game DROP COLUMN user_id;
ALTER TABLE singleplayer_game RENAME COLUMN new_user_id TO user_id;
ALTER TABLE singleplayer_game ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE singleplayer_game ADD FOREIGN KEY (user_id) REFERENCES user_info(id);

ALTER TABLE multiplayer_game DROP COLUMN creator_id;
ALTER TABLE multiplayer_game RENAME COLUMN new_creator_id TO creator_id;
ALTER TABLE multiplayer_game ALTER COLUMN creator_id SET NOT NULL;
ALTER TABLE multiplayer_game ADD FOREIGN KEY (creator_id) REFERENCES user_info(id);

ALTER TABLE multiplayer_game_user DROP COLUMN user_id;
ALTER TABLE multiplayer_game_user RENAME COLUMN new_user_id TO user_id;
ALTER TABLE multiplayer_game_user ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE multiplayer_game_user ADD FOREIGN KEY (user_id) REFERENCES user_info(id);

ALTER TABLE multiplayer_round_user DROP COLUMN user_id;
ALTER TABLE multiplayer_round_user RENAME COLUMN new_user_id TO user_id;
ALTER TABLE multiplayer_round_user ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE multiplayer_round_user ADD FOREIGN KEY (user_id) REFERENCES user_info(id);

-- Convert panorama_location
ALTER TABLE panorama_location ADD COLUMN new_id UUID DEFAULT uuidv7();

-- Update foreign keys pointing to panorama_location
ALTER TABLE singleplayer_round ADD COLUMN new_location_id UUID;
UPDATE singleplayer_round AS sr SET new_location_id = pl.new_id FROM panorama_location AS pl WHERE sr.location_id = pl.id;

ALTER TABLE multiplayer_round ADD COLUMN new_location_id UUID;
UPDATE multiplayer_round AS mr SET new_location_id = pl.new_id FROM panorama_location AS pl WHERE mr.location_id = pl.id;

-- Drop old constraints and columns for panorama_location
ALTER TABLE singleplayer_round DROP CONSTRAINT singleplayer_round_location_id_fkey;
ALTER TABLE multiplayer_round DROP CONSTRAINT multiplayer_round_location_id_fkey;

ALTER TABLE panorama_location DROP CONSTRAINT panorama_location_pkey CASCADE;
ALTER TABLE panorama_location DROP COLUMN id;
ALTER TABLE panorama_location RENAME COLUMN new_id TO id;
ALTER TABLE panorama_location ADD PRIMARY KEY (id);

-- Finalize panorama_location foreign keys
ALTER TABLE singleplayer_round DROP COLUMN location_id;
ALTER TABLE singleplayer_round RENAME COLUMN new_location_id TO location_id;
ALTER TABLE singleplayer_round ALTER COLUMN location_id SET NOT NULL;
ALTER TABLE singleplayer_round ADD FOREIGN KEY (location_id) REFERENCES panorama_location(id);

ALTER TABLE multiplayer_round DROP COLUMN location_id;
ALTER TABLE multiplayer_round RENAME COLUMN new_location_id TO location_id;
ALTER TABLE multiplayer_round ALTER COLUMN location_id SET NOT NULL;
ALTER TABLE multiplayer_round ADD FOREIGN KEY (location_id) REFERENCES panorama_location(id);

-- Convert singleplayer_game
ALTER TABLE singleplayer_game ADD COLUMN new_id UUID DEFAULT uuidv7();

-- Update foreign keys pointing to singleplayer_game
ALTER TABLE singleplayer_round ADD COLUMN new_game_id UUID;
UPDATE singleplayer_round AS sr SET new_game_id = sg.new_id FROM singleplayer_game AS sg WHERE sr.game_id = sg.id;

-- Drop old constraints and columns for singleplayer_game
ALTER TABLE singleplayer_round DROP CONSTRAINT singleplayer_round_game_id_fkey;

ALTER TABLE singleplayer_game DROP CONSTRAINT singleplayer_game_pkey CASCADE;
ALTER TABLE singleplayer_game DROP COLUMN id;
ALTER TABLE singleplayer_game RENAME COLUMN new_id TO id;
ALTER TABLE singleplayer_game ADD PRIMARY KEY (id);

-- Finalize singleplayer_game foreign keys
ALTER TABLE singleplayer_round DROP COLUMN game_id;
ALTER TABLE singleplayer_round RENAME COLUMN new_game_id TO game_id;
ALTER TABLE singleplayer_round ALTER COLUMN game_id SET NOT NULL;
ALTER TABLE singleplayer_round ADD FOREIGN KEY (game_id) REFERENCES singleplayer_game(id);

-- Convert multiplayer_game
ALTER TABLE multiplayer_game ADD COLUMN new_id UUID DEFAULT uuidv7();

-- Update foreign keys pointing to multiplayer_game
ALTER TABLE multiplayer_game_user ADD COLUMN new_game_id UUID;
UPDATE multiplayer_game_user mgu SET new_game_id = mg.new_id FROM multiplayer_game AS mg WHERE mgu.game_id = mg.id;

ALTER TABLE multiplayer_round ADD COLUMN new_game_id UUID;
UPDATE multiplayer_round AS mr SET new_game_id = mg.new_id FROM multiplayer_game AS mg WHERE mr.game_id = mg.id;

-- Drop old constraints and columns for multiplayer_game
ALTER TABLE multiplayer_game_user DROP CONSTRAINT multiplayer_game_user_game_id_fkey;
ALTER TABLE multiplayer_round DROP CONSTRAINT multiplayer_round_game_id_fkey;
ALTER TABLE multiplayer_game DROP CONSTRAINT multiplayer_game_pkey CASCADE;
ALTER TABLE multiplayer_game DROP COLUMN id;
ALTER TABLE multiplayer_game RENAME COLUMN new_id TO id;
ALTER TABLE multiplayer_game ADD PRIMARY KEY (id);

-- Finalize multiplayer_game foreign keys
ALTER TABLE multiplayer_game_user DROP COLUMN game_id;
ALTER TABLE multiplayer_game_user RENAME COLUMN new_game_id TO game_id;
ALTER TABLE multiplayer_game_user ALTER COLUMN game_id SET NOT NULL;
ALTER TABLE multiplayer_game_user ADD FOREIGN KEY (game_id) REFERENCES multiplayer_game(id);

ALTER TABLE multiplayer_round DROP COLUMN game_id;
ALTER TABLE multiplayer_round RENAME COLUMN new_game_id TO game_id;
ALTER TABLE multiplayer_round ALTER COLUMN game_id SET NOT NULL;
ALTER TABLE multiplayer_round ADD FOREIGN KEY (game_id) REFERENCES multiplayer_game(id);

-- Convert remaining tables (no foreign key dependencies)
ALTER TABLE user_oauth ADD COLUMN new_id UUID DEFAULT uuidv7();

ALTER TABLE user_oauth DROP CONSTRAINT user_oauth_pkey CASCADE;
ALTER TABLE user_oauth DROP COLUMN id;
ALTER TABLE user_oauth RENAME COLUMN new_id TO id;
ALTER TABLE user_oauth ADD PRIMARY KEY (id);

ALTER TABLE singleplayer_round ADD COLUMN new_id UUID DEFAULT uuidv7();

ALTER TABLE singleplayer_round DROP CONSTRAINT singleplayer_round_pkey CASCADE;
ALTER TABLE singleplayer_round DROP COLUMN id;
ALTER TABLE singleplayer_round RENAME COLUMN new_id TO id;
ALTER TABLE singleplayer_round ADD PRIMARY KEY (id);

ALTER TABLE singleplayer_round_guess ADD COLUMN new_id UUID DEFAULT uuidv7();

ALTER TABLE singleplayer_round_guess DROP CONSTRAINT singleplayer_round_guess_pkey CASCADE;
ALTER TABLE singleplayer_round_guess DROP COLUMN id;
ALTER TABLE singleplayer_round_guess RENAME COLUMN new_id TO id;
ALTER TABLE singleplayer_round_guess ADD PRIMARY KEY (id);

ALTER TABLE multiplayer_round ADD COLUMN new_id UUID DEFAULT uuidv7();

ALTER TABLE multiplayer_round DROP CONSTRAINT multiplayer_round_pkey CASCADE;
ALTER TABLE multiplayer_round DROP COLUMN id;
ALTER TABLE multiplayer_round RENAME COLUMN new_id TO id;
ALTER TABLE multiplayer_round ADD PRIMARY KEY (id);

ALTER TABLE multiplayer_round_user ADD COLUMN new_id UUID DEFAULT uuidv7();

ALTER TABLE multiplayer_round_user DROP CONSTRAINT multiplayer_round_user_pkey CASCADE;
ALTER TABLE multiplayer_round_user DROP COLUMN id;
ALTER TABLE multiplayer_round_user RENAME COLUMN new_id TO id;
ALTER TABLE multiplayer_round_user ADD PRIMARY KEY (id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Reverting is complex and may cause data loss
SELECT 'DOWN MIGRATION NOT SUPPORTED' AS notice;
-- +goose StatementEnd