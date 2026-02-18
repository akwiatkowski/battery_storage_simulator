.PHONY: build export-data serve dev clean

build:
	mise exec -- wasm-pack build --target web --out-dir www/pkg

export-data:
	cd .. && mise exec -- python battery_optimizer_wasm/scripts/export_data.py
	cp data/hourly.json www/data.json

serve:
	@echo "Serving at http://localhost:8000"
	cd www && python3 -m http.server 8000

dev: build export-data serve

clean:
	rm -rf www/pkg target data/hourly.json www/data.json
