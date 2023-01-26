CREATE TABLE archive
(
    id    serial PRIMARY KEY,
    query VARCHAR NOT NULL,
    data  VARCHAR NOT NULL
);

CREATE UNIQUE INDEX archive_query_ndx ON archive (query);