DROP TABLE IF EXISTS user;


CREATE TABLE user (
    id char(26) NOT NULL primary key,
    name varchar(50) NOT NULL,
    age int(3) NOT NULL
);

insert into user values ('00000000000000000000000001', 'hanako', 20);
insert into user values ('00000000000000000000000002', 'taro', 30);