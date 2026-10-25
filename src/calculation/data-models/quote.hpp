#pragma once

namespace dagobert {

class Quote {
public:
  static Quote require(double amount);

  [[nodiscard]] double amount() const { return amount_; }

private:
  explicit Quote(double amount);
  double amount_;
};

} // namespace dagobert
