
INSERT INTO panorama_location (streetview_id, lat, lng, provider)
SELECT ya_id, lat, lng, provider
FROM yandex_airview_temp;
