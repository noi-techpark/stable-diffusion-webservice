-- SPDX-FileCopyrightText: NOI Techpark <digital@noi.bz.it>
--
-- SPDX-License-Identifier: AGPL-3.0-or-later

begin transaction;

drop table if exists jobs;
create table jobs (
    created_ms      integer not null,
    prompt          text not null,
    number          text not null,
    width           integer not null,
    height          integer not null,
    token           text not null,
    state           text not null,
    completed_ms    integer,
    check(state in ('new', 'pending', 'complete'))
);

drop table if exists secrets;
create table secrets (
    kind        text not null,
    secret      text,
    unique(kind)
);

insert into secrets values('hcaptcha_secret', '');
insert into secrets values('hcaptcha_sitekey', '');
insert into secrets values('backend_secret', '');

commit;


