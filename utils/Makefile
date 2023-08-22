SHELL := /bin/bash
include ../mongo.pass.env
COLLECTIONS=pilots teams judges tricks competitions seasons files
BACKUP_DIR=mongo.backup.$(shell date +'%Y%m%d-%H%M')
BACKUP_FILE=$(BACKUP_DIR).tar.gz
to?=$(BACKUP_FILE)
TMPDIR := $(shell mktemp -d)

default:
	@echo "make backup: backup a database"
	@echo "  provide MONGODB_URL env"

export:
	@test -n "$(from)" || (echo "missing from parameter" ; false)
	@[[ "$(from)" == "PROD" || "$(from)" == "PREPROD" || "$(from)" == "DEV" || "$(from)" == "TRAINING" ]] || (echo "from parameter ($(from)) must be PROD, PREPROD, TRAINING or DEV" ; false)

	@[[ ! -f "$(BACKUP_FILE)" ]] || (echo "$(BACKUP_FILE) already exist"; false)

	@echo "exporting from $(from) DB to $(TMPDIR) ..."
	@for collection in $(COLLECTIONS); do mongoexport --uri="$(ACROPYX_MONGO_$(from))" --collection=$${collection} --out=$(TMPDIR)/$${collection}.json; done

	@tar -C $(TMPDIR) -czf $(to) -C $(TMPDIR) .
	@echo "Mongo $(from) database backuped up to $(to)"

import:
	@test -n "$(from)" || (echo "missing from parameter" ; false)
	@[[ "$(to)" == "PROD" || "$(to)" == "PREPROD" || "$(to)" == "DEV" || "$(to)" == "TRAINING" ]] || (echo "to parameter ($(to)) must be PROD, PREPROD, TRAINING or DEV" ; false)

	@[[ -f "$(from)" ]] || (echo "backup file $(from) not found"; false)

	@echo "importing from $(from) to $(to DB) ..."
	@tar -C $(TMPDIR) -xzf $(from)

	for collection in $(COLLECTIONS); do mongosh $(ACROPYX_MONGO_$(to)) --eval "db.$${collection}.deleteMany({})" ; mongoimport --uri="$(ACROPYX_MONGO_$(to))" --collection=$${collection} $(TMPDIR)/$${collection}.json; done

	@echo "Mongo database from backup file ($(from)) imported to $(to) DB"
