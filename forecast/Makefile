ROOT := $(shell cd .. && pwd)
PYTHON := python3

.PHONY: setup fetch-weather \
       train-pv train-consumption train-heat-pump train-dhw train-spot-price train-all \
       evaluate-pv evaluate-consumption evaluate-heat-pump evaluate-dhw evaluate-spot-price evaluate-all \
       predict test clean

setup:
	$(PYTHON) -m pip install -r requirements.txt

fetch-weather:
	cd $(ROOT) && $(PYTHON) -m forecast.src.fetch_weather

# Training targets
train-pv: fetch-weather
	cd $(ROOT) && $(PYTHON) -m forecast.src.train --model pv

train-consumption: fetch-weather
	cd $(ROOT) && $(PYTHON) -m forecast.src.train --model consumption

train-heat-pump: fetch-weather
	cd $(ROOT) && $(PYTHON) -m forecast.src.train --model heat_pump

train-dhw:
	cd $(ROOT) && $(PYTHON) -m forecast.src.train --model dhw

train-spot-price: fetch-weather
	cd $(ROOT) && $(PYTHON) -m forecast.src.train --model spot_price

train-all: train-pv train-consumption train-heat-pump train-dhw train-spot-price

# Evaluation targets
evaluate-pv: train-pv
	cd $(ROOT) && $(PYTHON) -m forecast.src.evaluate --model pv

evaluate-consumption: train-consumption
	cd $(ROOT) && $(PYTHON) -m forecast.src.evaluate --model consumption

evaluate-heat-pump: train-heat-pump
	cd $(ROOT) && $(PYTHON) -m forecast.src.evaluate --model heat_pump

evaluate-dhw: train-dhw
	cd $(ROOT) && $(PYTHON) -m forecast.src.evaluate --model dhw

evaluate-spot-price: train-spot-price
	cd $(ROOT) && $(PYTHON) -m forecast.src.evaluate --model spot_price

evaluate-all: evaluate-pv evaluate-consumption evaluate-heat-pump evaluate-dhw evaluate-spot-price

# Predict
predict:
	cd $(ROOT) && $(PYTHON) -m forecast.src.predict

# Test
test:
	cd $(ROOT) && $(PYTHON) -m pytest forecast/tests/ -v

clean:
	rm -rf models/ docs/forecast/
