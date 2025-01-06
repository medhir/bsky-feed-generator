CREATE TABLE IF NOT EXISTS post(
    record varchar(59) primary key not null,
    uri varchar(76) not null,
    did varchar(32) not null,
    indexed_at timestamptz not null,
    nsfw boolean not null,
    likes integer not null default 0,
    reposts integer not null default 0,
    tt_confidence float not null
)