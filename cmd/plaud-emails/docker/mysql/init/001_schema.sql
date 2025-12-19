-- 在test库中初始化users表
CREATE TABLE IF NOT EXISTS users (
    id BIGINT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    address VARCHAR(255) NOT NULL,
    ct BIGINT NOT NULL,
    ut BIGINT NOT NULL,
    ver INT NOT NULL,
    status TINYINT NOT NULL,
    del TINYINT NOT NULL
);