class Lazyspeed < Formula
  desc "Terminal-based internet speed test"
  homepage "https://github.com/jkleinne/lazyspeed"
  url "https://github.com/jkleinne/lazyspeed/archive/v0.1.2.tar.gz"
  sha256 "910c07cab8fbb0e92bb9aa4579da23ab941e6256afe76156f64c602646413c15"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "mod", "download"
    system "go", "build", *std_go_args, "."
  end

  test do
    assert_match "version #{version}", shell_output("#{bin}/lazyspeed version")
  end
end