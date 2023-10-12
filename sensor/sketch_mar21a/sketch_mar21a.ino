#include <WiFi.h>
#include <hp_BH1750.h>
#include <InfluxDbClient.h>
#include "secrets.h"

// WiFi
WiFiClient wifiClient;

// connect to DB
InfluxDBClient dbClient(INFLUXDB_URL, INFLUXDB_ORG, INFLUXDB_BUCKET, INFLUXDB_TOKEN);

// create InfluxDB point (which will be updated and sent every loop)
Point lightSensor("ambient_light_level");

// create ambient light sensor
hp_BH1750 sensor;

void setup()
{
  // open serial
  Serial.begin(115200);

  // connect to WiFi
  Serial.print("attempting to connect to SSID: ");
  Serial.println(WIFI_SSID);

  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);
  while (WiFi.status() != WL_CONNECTED) {
      delay(500);
      Serial.print(".");
  }

  Serial.println("");
  Serial.println("connected to WiFi");

  // set Wire1 (second wire) pins so we can connect to the BH1750 sensor
  Wire1.setPins(SDA1, SCL1);

  // init light sensor
  sensor.begin(BH1750_TO_GROUND, &Wire1); // <- needed because sensor is connected to QUIC port
  
  // make first measurement
  sensor.start();

  // add lightSensor tag
  lightSensor.addTag("device", "living-room-1");
}

void loop()
{
  // check if sensor value is ready
  if (sensor.hasValue()) {
    // get and print lux
    float lux = sensor.getLux();
    Serial.printf("%f lux\n", lux);

    // save in InfluxDB
    lightSensor.clearFields();
    lightSensor.addField("intensity", lux);

    // print whether write was successful
    if (!dbClient.writePoint(lightSensor)) {
      Serial.print("influxdb write failed: ");
      Serial.println(dbClient.getLastErrorMessage());
    } else {
      Serial.println("added point to database");
    }
    Serial.println("");

    // start reading next point
    sensor.start();
  }
}