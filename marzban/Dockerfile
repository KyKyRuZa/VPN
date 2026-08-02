FROM gozargah/marzban:v0.8.4

USER root
RUN pip install --no-cache-dir psycopg2-binary==2.9.9
RUN useradd -m -u 1000 marzban
RUN mkdir -p /usr/local/bin/xray-core
# Marzban/Xray writes xray_config.json into /code at runtime; grant write access
RUN chown -R marzban:marzban /code
RUN chown marzban:marzban /usr/local/bin/xray-core

COPY entrypoint.sh /marzban/entrypoint.sh
RUN chmod +x /marzban/entrypoint.sh
ENTRYPOINT ["/marzban/entrypoint.sh"]

COPY xray_config.json /code/xray_config.json

RUN python - <<'EOF'
import pathlib

# Patch 1: include_db_users() PostgreSQL fixes
p = pathlib.Path("/code/app/xray/config.py")
s = p.read_text()
s = s.replace("\r\n", "\n")

assert "from app.db import models as db_models" in s
s = s.replace(
    "from app.db import models as db_models\n",
    "from app.db import models as db_models\nfrom app.db.base import IS_SQLITE\n",
)
assert "from sqlalchemy import func\n" in s
s = s.replace(
    "from sqlalchemy import func\n",
    "from sqlalchemy import func, literal, Text\n",
)

old = "func.group_concat(db_models.excluded_inbounds_association.c.inbound_tag).label('excluded_inbound_tags')"
new = (
    "(func.string_agg(db_models.excluded_inbounds_association.c.inbound_tag, literal(',')) "
    "if not IS_SQLITE else "
    "func.group_concat(db_models.excluded_inbounds_association.c.inbound_tag))"
    ".label('excluded_inbound_tags')"
)
assert old in s, "group_concat pattern not found"
s = s.replace(old, new)

old_lower = "func.lower(db_models.Proxy.type)"
new_lower = "func.lower(db_models.Proxy.type.cast(Text))"
assert s.count(old_lower) == 2, f"expected 2 lower(Proxy.type), found {s.count(old_lower)}"
s = s.replace(old_lower, new_lower)

old_grp = (
    "                db_models.User.username,\n"
    "                db_models.Proxy.settings,\n"
)
new_grp = (
    "                db_models.User.username,\n"
    "                db_models.Proxy.id,\n"
)
assert s.count(old_grp) == 1, f"expected 1 group_by settings, found {s.count(old_grp)}"
s = s.replace(old_grp, new_grp)

p.write_text(s)
print("patched marzban include_db_users for postgres")

# Patch 2: h3->alpn enum migration PostgreSQL fixes
p = pathlib.Path("/code/app/db/migrations/versions/305943d779c4_add_h3_to_alpn_enum.py")
s = p.read_text()

assert 'enum_name = "alpn"' in s
s = s.replace('enum_name = "alpn"', 'enum_name = "proxyhostalpn"')

idx = s.index("def upgrade():")
head = s[:idx]
tail = '''
def upgrade():
    bind = op.get_bind()
    is_postgres = bind.dialect.name == "postgresql"
    if is_postgres:
        op.execute(f"ALTER TABLE {table_name} ALTER COLUMN {column_name} DROP DEFAULT;")

    temp_type.create(op.get_bind(), checkfirst=False)

    with op.batch_alter_table(table_name) as batch_op:
        batch_op.alter_column(
            column_name,
            existing_type=old_type,
            type_=temp_type,
            existing_nullable=False,
            postgresql_using=f"{column_name}::text::{temp_enum_name}",
        )

    old_type.drop(op.get_bind(), checkfirst=False)
    new_type.create(op.get_bind(), checkfirst=False)

    with op.batch_alter_table(table_name) as batch_op:
        batch_op.alter_column(
            column_name,
            existing_type=temp_type,
            type_=new_type,
            existing_nullable=False,
            postgresql_using=f"{column_name}::text::{enum_name}",
        )

    temp_type.drop(op.get_bind(), checkfirst=False)

    if is_postgres:
        op.execute(f"ALTER TABLE {table_name} ALTER COLUMN {column_name} SET DEFAULT 'none';")


def downgrade():
    update_query = (
        temp_table
        .update()
        .where(temp_table.c.alpn.in_(downgrade_from))
        .values(alpn=downgrade_to)
    )
    op.execute(update_query)

    bind = op.get_bind()
    is_postgres = bind.dialect.name == "postgresql"
    if is_postgres:
        op.execute(f"ALTER TABLE {table_name} ALTER COLUMN {column_name} DROP DEFAULT;")

    temp_type.create(op.get_bind(), checkfirst=False)

    with op.batch_alter_table(table_name) as batch_op:
        batch_op.alter_column(
            column_name,
            existing_type=new_type,
            type_=temp_type,
            existing_nullable=False,
            postgresql_using=f"{column_name}::text::{temp_enum_name}",
        )

    new_type.drop(op.get_bind(), checkfirst=False)
    old_type.create(op.get_bind(), checkfirst=False)

    with op.batch_alter_table(table_name) as batch_op:
        batch_op.alter_column(
            column_name,
            existing_type=temp_type,
            type_=old_type,
            existing_nullable=False,
            postgresql_using=f"{column_name}::text::{enum_name}",
        )

    temp_type.drop(op.get_bind(), checkfirst=False)

    if is_postgres:
        op.execute(f"ALTER TABLE {table_name} ALTER COLUMN {column_name} SET DEFAULT 'none';")
'''
assert "def upgrade():" in s
s = head + tail
p.write_text(s)
print("patched marzban alpn enum migration for postgres")

# Patch 3: last_status_change migration PostgreSQL fix
p = pathlib.Path("/code/app/db/migrations/versions/54c4b8c525fc_last_status_change_for_expired_users.py")
s = p.read_text()

old = (
    "    else:\n"
    "        # For SQLite: Use DATETIME with 'unixepoch'\n"
)
new = (
    "    elif connection.engine.name == \"postgresql\":\n"
    "        update_stmt = (\n"
    "            sa.update(users_table)\n"
    "            .where(\n"
    "                sa.and_(\n"
    "                    users_table.c.status == 'expired',\n"
    "                    users_table.c.expire.isnot(None)\n"
    "                )\n"
    "            )\n"
    "            .values(last_status_change=func.to_timestamp(users_table.c.expire))\n"
    "        )\n"
    "    else:\n"
    "        # For SQLite: Use DATETIME with 'unixepoch'\n"
)
assert old in s, "last_status_change else-block not found"
s = s.replace(old, new)
p.write_text(s)
print("patched marzban last_status_change migration for postgres")
EOF

USER marzban
