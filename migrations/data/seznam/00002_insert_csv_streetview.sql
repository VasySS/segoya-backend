
INSERT INTO panorama_location (lat, lng, provider)
SELECT lat, lng, provider
FROM seznam_temp;
