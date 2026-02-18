ROOT := $(shell cd ../.. && pwd)
PYTHON := python3

.PHONY: setup fetch-weather train-pv evaluate-pv predict test clean

setup:
	$(PYTHON) -m pip install -r requirements.txt

fetch-weather:
	cd $(ROOT) && $(PYTHON) -m analysis.python.src.fetch_weather

train-pv: fetch-weather
	cd $(ROOT) && $(PYTHON) -m analysis.python.src.train --model pv

evaluate-pv: train-pv
	cd $(ROOT) && $(PYTHON) -m analysis.python.src.evaluate --model pv

predict:
	cd $(ROOT) && $(PYTHON) -m analysis.python.src.predict

test:
	cd $(ROOT) && $(PYTHON) -m pytest analysis/python/tests/ -v

clean:
	rm -rf models/ output/
