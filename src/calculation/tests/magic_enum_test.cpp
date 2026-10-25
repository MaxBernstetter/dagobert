#include <gtest/gtest.h>
#include <magic_enum/magic_enum.hpp>

namespace {

enum class SampleColor { red, green };

} // namespace

TEST(MagicEnum, EnumNameMatchesEnumeratorIdentifier) {
  EXPECT_EQ(magic_enum::enum_name(SampleColor::red), "red");
}
