
INSERT INTO panorama_location (lat, lng, provider)
SELECT lat, lng, provider
FROM yandex_streetview_temp;
