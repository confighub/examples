package com.example.orderapi;

import com.example.orderapi.api.OrderApiFeatureProperties;
import com.example.orderapi.api.OrderApiRuntimeProperties;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.boot.context.properties.EnableConfigurationProperties;

@SpringBootApplication
@EnableConfigurationProperties({
    OrderApiFeatureProperties.class,
    OrderApiRuntimeProperties.class
})
public class OrderApiApplication {

  public static void main(String[] args) {
    SpringApplication.run(OrderApiApplication.class, args);
  }
}
