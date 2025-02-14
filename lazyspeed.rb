class Lazyspeed < Formula
  desc "Terminal-based internet speed test"
  homepage "https://github.com/jkleinne/lazyspeed"
  url "https://github.com/jkleinne/lazyspeed/archive/v0.1.0.tar.gz"
  sha256 "ccf193eb8d0317f25124e8aa34baa45f8765f30a59744e02ef9fe649b2c05341"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "mod", "download"
    system "go", "build", *std_go_args, "./main.go"
  end

  test do
    
  end
end