CREATE TABLE IF NOT EXISTS bird_post(
    rkey varchar(59) primary key not null,
    uri varchar(76) not null,
    did varchar(32) not null,
    indexed_at timestamptz not null
);

CREATE TABLE IF NOT EXISTS potential_bird_post(
    rkey varchar(59) primary key not null,
    uri varchar(76) not null,
    did varchar(32) not null,
    indexed_at timestamptz not null
);

CREATE TABLE IF NOT EXISTS post_like(
    record varchar(59) primary key not null,
    post_rkey varchar(32) not null,
    did varchar(32) not null,
    indexed_at timestamptz not null
);

CREATE TABLE IF NOT EXISTS post_repost(
    record varchar(59) primary key not null,
    post_rkey varchar(59) not null,
    did varchar(32) not null,
    indexed_at timestamptz not null
);