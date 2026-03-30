package com.example.inventory;

import com.example.inventory.api.InventoryFeatureProperties;
import com.example.inventory.api.InventoryRuntimeProperties;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.boot.context.properties.EnableConfigurationProperties;

@SpringBootApplication
@EnableConfigurationProperties({
    InventoryFeatureProperties.class,
    InventoryRuntimeProperties.class
})
public class InventoryApiApplication {

  public static void main(String[] args) {
    SpringApplication.run(InventoryApiApplication.class, args);
  }
}
